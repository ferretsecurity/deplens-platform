package socket

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("a Socket sync is already active")
var ErrConfig = errors.New("Socket organization and token are required")
var ErrLease = errors.New("Socket sync ownership expired")
var organizationPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,99}$`)
var githubRepositoryPathPattern = regexp.MustCompile(`^/[a-zA-Z0-9][a-zA-Z0-9-]*/[a-zA-Z0-9_.-]+/?$`)

type Run struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Total      int        `json:"total"`
	Completed  int        `json:"completed"`
	Failed     int        `json:"failed"`
	Message    string     `json:"message"`
}
type Configuration struct {
	Enabled         bool       `json:"enabled"`
	Organization    string     `json:"organization"`
	TokenConfigured bool       `json:"token_configured"`
	Interval        int        `json:"interval_minutes"`
	NextRun         *time.Time `json:"next_run_at"`
	LastRun         *Run       `json:"last_run"`
}
type Service struct {
	DB     *pgxpool.Pool
	Client *Client
	Vault  *Vault
}

func (s *Service) Configuration(ctx context.Context, tenant string) (Configuration, error) {
	result := Configuration{Enabled: s.Vault != nil}
	err := s.DB.QueryRow(ctx, `select organization,octet_length(token_cipher)>0,interval_minutes,next_run_at from socket_connections where tenant_id=$1`, tenant).Scan(&result.Organization, &result.TokenConfigured, &result.Interval, &result.NextRun)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	var run Run
	err = s.DB.QueryRow(ctx, `select r.id,r.status,r.started_at,r.finished_at,r.total,r.completed,r.failed,r.message from socket_runs r join socket_connections c on c.id=r.connection_id where c.tenant_id=$1 order by r.started_at desc,r.id desc limit 1`, tenant).Scan(&run.ID, &run.Status, &run.StartedAt, &run.FinishedAt, &run.Total, &run.Completed, &run.Failed, &run.Message)
	if err == nil {
		result.LastRun = &run
	}
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return result, err
}
func validInterval(n int) bool { return n == 0 || n == 60 || n == 360 || n == 720 || n == 1440 }
func (s *Service) Save(ctx context.Context, tenant, org string, token *string, interval int) error {
	if s.Vault == nil {
		return errors.New("Socket integration is disabled; configure the deployment encryption key")
	}
	if !organizationPattern.MatchString(org) || !validInterval(interval) {
		return ErrConfig
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// The tenant lock serializes first-time configuration.
	if _, err = tx.Exec(ctx, `select id from tenants where id=$1 for update`, tenant); err != nil {
		return err
	}
	var oldOrg string
	var cipher []byte
	err = tx.QueryRow(ctx, `select organization,token_cipher from socket_connections where tenant_id=$1 for update`, tenant).Scan(&oldOrg, &cipher)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var active bool
	if err = tx.QueryRow(ctx, `select exists(select 1 from socket_runs r join socket_connections c on c.id=r.connection_id where c.tenant_id=$1 and r.status in ('queued','running'))`, tenant).Scan(&active); err != nil {
		return err
	}
	if active {
		return ErrConflict
	}
	if oldOrg != "" && oldOrg != org {
		return errors.New("disconnect before changing the Socket organization")
	}
	if token != nil {
		if len(*token) == 0 || len(*token) > 4096 {
			return ErrConfig
		}
		cipher, err = s.Vault.Seal(*token, tenant)
		if err != nil {
			return err
		}
	}
	if len(cipher) == 0 {
		return ErrConfig
	}
	_, err = tx.Exec(ctx, `insert into socket_connections(tenant_id,organization,token_cipher,interval_minutes,next_run_at) values($1,$2,$3,$4,case when $4>0 then now()+make_interval(mins=>$4) end) on conflict(tenant_id) do update set token_cipher=excluded.token_cipher,interval_minutes=excluded.interval_minutes,next_run_at=excluded.next_run_at`, tenant, org, cipher, interval)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) Disconnect(ctx context.Context, tenant string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `select id from tenants where id=$1 for update`, tenant); err != nil {
		return err
	}
	// Lock and cancel runs before deleting, fencing any in-flight publication.
	if _, err = tx.Exec(ctx, `update socket_runs r set status='cancelled',finished_at=now(),owner=null,lease_until=null from socket_connections c where c.id=r.connection_id and c.tenant_id=$1 and r.status in ('queued','running')`, tenant); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `delete from socket_connections where tenant_id=$1`, tenant); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) credentials(ctx context.Context, tenant string) (string, string, error) {
	if s.Vault == nil {
		return "", "", ErrConfig
	}
	var org string
	var data []byte
	if err := s.DB.QueryRow(ctx, `select organization,token_cipher from socket_connections where tenant_id=$1`, tenant).Scan(&org, &data); err != nil {
		return "", "", ErrConfig
	}
	token, err := s.Vault.Open(data, tenant)
	return org, token, err
}
func (s *Service) Test(ctx context.Context, tenant string) error {
	org, token, err := s.credentials(ctx, tenant)
	if err != nil {
		return err
	}
	_, err = s.Client.Repositories(ctx, org, token)
	if err != nil {
		return err
	}
	return s.Client.get(ctx, orgPath(org)+"/full-scans?per_page=1", token, "application/json", func(r io.Reader) error { _, err := io.Copy(io.Discard, r); return err })
}
func (s *Service) Start(ctx context.Context, tenant string) (string, error) {
	if s.Vault == nil {
		return "", ErrConfig
	}
	var id string
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var connection string
	if err = tx.QueryRow(ctx, `select id from socket_connections where tenant_id=$1 for update`, tenant).Scan(&connection); err != nil {
		return "", ErrConfig
	}
	err = tx.QueryRow(ctx, `insert into socket_runs(connection_id,status) values($1,'queued') on conflict(connection_id) where status in ('queued','running') do nothing returning id`, connection).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		cfg, e := s.Configuration(ctx, tenant)
		if e != nil {
			return "", e
		}
		if cfg.TokenConfigured {
			return "", ErrConflict
		}
		return "", ErrConfig
	}
	if err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}
func (s *Service) MapRepository(ctx context.Context, tenant, id string, cli *string) error {
	if _, err := uuid.Parse(id); err != nil {
		return ErrConfig
	}
	if cli != nil {
		if _, err := uuid.Parse(*cli); err != nil {
			return ErrConfig
		}
	}
	tag, err := s.DB.Exec(ctx, `update socket_repositories sr set cli_repository_id=$3,mapping_explicit=true from socket_connections c where sr.connection_id=c.id and c.tenant_id=$1 and sr.id=$2 and ($3::uuid is null or exists(select 1 from repositories r where r.id=$3 and r.tenant_id=$1))`, tenant, id, cli)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrConfig
	}
	return err
}
func (s *Service) Work(ctx context.Context) {
	if s.Vault == nil {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		s.workOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Service) workOnce(ctx context.Context) {
	// An insert coalesces missed intervals. The unique partial index serializes starts across replicas.
	_, err := s.DB.Exec(ctx, `with due as materialized (select id from socket_connections where interval_minutes>0 and next_run_at<=now() for update) insert into socket_runs(connection_id,status) select id,'queued' from due on conflict(connection_id) where status in ('queued','running') do nothing`)
	if err != nil {
		return
	}
	owner := uuid.NewString()
	var runID, connection, tenant, org string
	var cipher []byte
	err = s.DB.QueryRow(ctx, `with candidate as (select id from socket_runs where status='queued' or (status='running' and lease_until<now()) order by started_at for update skip locked limit 1) update socket_runs r set status='running',owner=$1,lease_until=now()+interval '60 seconds',total=0,completed=0,failed=0,message='' from candidate,socket_connections c where r.id=candidate.id and c.id=r.connection_id returning r.id,c.id,c.tenant_id,c.organization,c.token_cipher`, owner).Scan(&runID, &connection, &tenant, &org, &cipher)
	if err != nil {
		return
	}
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	var heartbeat sync.WaitGroup
	heartbeat.Add(1)
	go func() {
		defer heartbeat.Done()
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-tick.C:
				tag, e := s.DB.Exec(runCtx, `update socket_runs set lease_until=now()+interval '60 seconds' where id=$1 and owner=$2 and status='running' and lease_until>now()`, runID, owner)
				if e != nil || tag.RowsAffected() != 1 {
					cancel()
					return
				}
			}
		}
	}()
	token, err := s.Vault.Open(cipher, tenant)
	if err == nil {
		err = s.sync(runCtx, runID, owner, connection, tenant, org, token)
	}
	status, message := "completed", ""
	if err != nil {
		status = "failed"
		message = "Sync could not finish. Check the connection and try again."
	}
	if ctx.Err() != nil {
		status = "cancelled"
		message = "Server stopped; sync again to continue."
	}
	cancel()
	heartbeat.Wait()
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	_, _ = s.DB.Exec(finishCtx, `with finished as (
 update socket_runs set status=case when $3='completed' and completed<total then 'failed' when $3='completed' and failed>0 then 'partial' else $3 end,message=$4,finished_at=now(),lease_until=null
 where id=$1 and owner=$2 and status='running' and lease_until>now() returning connection_id
 ) update socket_connections c set next_run_at=case when interval_minutes>0 then now()+make_interval(mins=>interval_minutes) end from finished f where c.id=f.connection_id`, runID, owner, status, message)
}
func (s *Service) sync(ctx context.Context, runID, owner, connection, tenant, org, token string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	repos, err := s.Client.Repositories(ctx, org, token)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(repos))
	for _, repo := range repos {
		ids = append(ids, repo.ID)
	}
	// Publish listing availability only after all pages have completed, under the run fence.
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = checkLease(ctx, tx, runID, owner); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `update socket_runs set total=$3 where id=$1 and owner=$2`, runID, owner, len(repos)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `update socket_repositories set available=(remote_id=any($2::text[])) where connection_id=$1`, connection, ids); err != nil {
		return err
	}
	for _, repo := range repos {
		if _, err = tx.Exec(ctx, `insert into socket_repositories(connection_id,remote_id,name,dashboard_url,default_branch) values($1,$2,$3,$4,$5) on conflict(connection_id,remote_id) do update set name=excluded.name,dashboard_url=excluded.dashboard_url,default_branch=excluded.default_branch,available=true`, connection, repo.ID, repo.Name, DashboardURL(repo.DashboardURL), repo.Branch); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	jobs := make(chan Repository, 4)
	progressErrors := make(chan error, 4)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for repo := range jobs {
				var snapshot Snapshot
				var e error
				if repo.Head == "" {
					e = errors.New("No head scan is available in Socket")
				} else {
					snapshot, e = s.Client.Snapshot(ctx, org, token, repo.Head)
				}
				if ctx.Err() != nil {
					return
				}
				if e == nil {
					e = s.publish(ctx, runID, owner, connection, tenant, repo, snapshot)
				}
				message := ""
				if e != nil {
					message = "Repository import failed; previous inventory retained."
					if errors.Is(e, ErrProcessing) {
						message = ErrProcessing.Error()
					}
					if repo.Head == "" {
						message = "No head scan is available in Socket."
					}
				}
				// Error details remain bounded and never contain raw upstream responses.

				failed := 0
				if e != nil {
					failed = 1
				}
				tag, progressErr := s.DB.Exec(ctx, `with owned as materialized (
 select id from socket_runs where id=$1 and owner=$2 and status='running' and lease_until>now() for update
 ), recorded as (
 update socket_repositories sr set last_error=$5 where sr.connection_id=$3 and sr.remote_id=$4 and exists(select 1 from owned) returning sr.id
 ) update socket_runs set completed=completed+1,failed=failed+$6 where id=$1 and owner=$2 and status='running' and lease_until>now() and exists(select 1 from recorded)`, runID, owner, connection, repo.ID, message, failed)
				if progressErr == nil && tag.RowsAffected() != 1 {
					progressErr = ErrLease
				}
				if progressErr != nil {
					select {
					case progressErrors <- progressErr:
					default:
					}
					cancel()
					return
				}
			}
		}()
	}
enqueue:
	for _, repo := range repos {
		select {
		case jobs <- repo:
		case <-ctx.Done():
			break enqueue
		}
	}
	close(jobs)
	workers.Wait()
	select {
	case err := <-progressErrors:
		return err
	default:
	}
	return ctx.Err()
}
func checkLease(ctx context.Context, tx pgx.Tx, id, owner string) error {
	var valid bool
	err := tx.QueryRow(ctx, `select owner=$2::uuid and status='running' and lease_until>now() from socket_runs where id=$1 for update`, id, owner).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrLease
	}
	return nil
}
func (s *Service) publish(ctx context.Context, runID, owner, connection, tenant string, repo Repository, snapshot Snapshot) error {
	scmURL := snapshot.Metadata.SCMURL
	if scmURL == "" {
		scmURL = githubRepositoryHomepage(repo.Homepage)
	}
	sort.Strings(snapshot.Files)
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = checkLease(ctx, tx, runID, owner); err != nil {
		return err
	}
	var match *string
	normalized := NormalizeSCM(scmURL)
	if normalized != "" {
		rows, e := tx.Query(ctx, `select id,url from repositories where tenant_id=$1`, tenant)
		if e != nil {
			return e
		}
		count := 0
		for rows.Next() {
			var id, raw string
			if e = rows.Scan(&id, &raw); e != nil {
				rows.Close()
				return e
			}
			if NormalizeSCM(raw) == normalized {
				idCopy := id
				match = &idCopy
				count++
			}
		}
		rows.Close()
		if rows.Err() != nil {
			return rows.Err()
		}
		if count != 1 {
			match = nil
		}
	}
	_, err = tx.Exec(ctx, `update socket_repositories set snapshot=$3,synced_at=now(),scan_id=$4,scm_url=$5,cli_repository_id=case when mapping_explicit then cli_repository_id else $6 end where connection_id=$1 and remote_id=$2`, connection, repo.ID, data, repo.Head, scmURL, match)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func githubRepositoryHomepage(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || strings.ContainsAny(raw, "?#") || u.RawPath != "" || !githubRepositoryPathPattern.MatchString(u.Path) {
		return ""
	}
	name := strings.TrimSuffix(path.Base(u.Path), ".git")
	if name == "" || name == "." || name == ".." {
		return ""
	}
	return raw
}

type InventoryRepository struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Branch    string `json:"default_branch"`
	CLI       bool   `json:"cli"`
	Socket    bool   `json:"socket"`
	SocketURL string `json:"socket_url"`
}
type Inventory struct {
	Repository InventoryRepository `json:"repository"`
	Sources    []Source            `json:"sources"`
}
type Source struct {
	ID        string     `json:"id"`
	Available bool       `json:"available"`
	Error     string     `json:"error"`
	SyncedAt  *time.Time `json:"synced_at"`
	Snapshot  *Snapshot  `json:"snapshot"`
}

func (s *Service) Inventory(ctx context.Context, tenant, id string) (Inventory, error) {
	result := Inventory{Sources: []Source{}}
	err := s.DB.QueryRow(ctx, `select id,name,url,default_branch,cli,socket,socket_url from inventory_repositories where tenant_id=$1 and id=$2`, tenant, id).Scan(&result.Repository.ID, &result.Repository.Name, &result.Repository.URL, &result.Repository.Branch, &result.Repository.CLI, &result.Repository.Socket, &result.Repository.SocketURL)
	if err != nil {
		return result, err
	}
	rows, err := s.DB.Query(ctx, `select sr.id,sr.available,sr.last_error,sr.synced_at,sr.snapshot from socket_repositories sr join socket_connections c on c.id=sr.connection_id where c.tenant_id=$1 and (sr.id=$2 or sr.cli_repository_id=$2) order by sr.id`, tenant, id)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var source Source
		var data []byte
		if err = rows.Scan(&source.ID, &source.Available, &source.Error, &source.SyncedAt, &data); err != nil {
			return result, err
		}
		if data != nil {
			if err = json.Unmarshal(data, &source.Snapshot); err != nil {
				return result, err
			}
		}
		result.Sources = append(result.Sources, source)
	}
	return result, rows.Err()
}
