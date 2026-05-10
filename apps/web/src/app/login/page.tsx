import { LoginForm } from "@/components/login-form";

export default function LoginPage() {
  return (
    <main className="relative min-h-screen overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(15,23,42,0.08),transparent_26%),radial-gradient(circle_at_bottom_right,rgba(14,165,233,0.14),transparent_28%)]" />
      <div className="relative mx-auto grid min-h-screen max-w-6xl items-center gap-10 px-4 py-10 md:grid-cols-[1.1fr_0.9fr] md:px-8 lg:px-10">
        <section className="space-y-6">
          <div className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white/80 px-3 py-1 text-xs font-medium uppercase tracking-[0.18em] text-slate-500 backdrop-blur">
            Self-hosted product UI
          </div>
          <h1 className="max-w-2xl text-5xl font-semibold tracking-tight text-slate-950 md:text-6xl">
            See scan results and dependency trees without leaving the product.
          </h1>
          <p className="max-w-xl text-lg leading-8 text-slate-600">
            `deplens` keeps the backend authoritative and gives customers a focused web app for browsing repositories, scan history, and manifests.
          </p>
          <div className="grid max-w-2xl gap-4 sm:grid-cols-3">
            {[
              ["Single origin", "No CORS split between UI and API"],
              ["Session auth", "Secure cookie-backed browser login"],
              ["Compose first", "Built to run cleanly on one server"]
            ].map(([title, body]) => (
              <div key={title} className="rounded-2xl border border-slate-200/80 bg-white/85 p-4 shadow-sm backdrop-blur">
                <div className="font-medium text-slate-900">{title}</div>
                <div className="mt-1 text-sm leading-6 text-slate-600">{body}</div>
              </div>
            ))}
          </div>
        </section>

        <section className="relative">
          <div className="absolute inset-0 -z-10 rounded-[2rem] bg-white/50 blur-3xl" />
          <LoginForm />
        </section>
      </div>
    </main>
  );
}
