import * as React from "react";

import { cn } from "@/lib/utils";

type TableProps = React.ComponentProps<"table"> & { layout?: "table" | "reflow" };
type TableCellProps = React.ComponentProps<"td"> & { mobileLabel?: string };

function Table({ layout = "table", className, ...props }: TableProps) {
  const table = <table role="table" className={cn("w-full caption-bottom text-sm", layout === "reflow" && "table-reflow", className)} {...props} />;
  return layout === "reflow" ? <div className="table-container">{table}</div> : table;
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return <thead role="rowgroup" className={cn("[&_tr]:border-b", className)} {...props} />;
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return <tbody role="rowgroup" className={cn("[&_tr:last-child]:border-0", className)} {...props} />;
}

function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return <tfoot className={cn("border-t bg-muted/50 font-medium [&>tr]:last:border-b-0", className)} {...props} />;
}

function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return <tr role="row" className={cn("border-b transition-colors hover:bg-muted/50 data-[state=selected]:bg-muted", className)} {...props} />;
}

function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return <th role="columnheader" scope="col" className={cn("h-12 px-4 text-left align-middle font-medium text-muted-foreground [&:has([role=checkbox])]:pr-0", className)} {...props} />;
}

function TableCell({ mobileLabel, children, className, ...props }: TableCellProps) {
  return (
    <td role="cell" className={cn("p-4 align-middle [&:has([role=checkbox])]:pr-0", className)} {...props}>
      {mobileLabel ? <span className="table-mobile-label" aria-hidden="true">{mobileLabel}</span> : null}
      {children}
    </td>
  );
}

function TableCaption({ className, ...props }: React.ComponentProps<"caption">) {
  return <caption className={cn("mt-4 text-sm text-muted-foreground", className)} {...props} />;
}

export {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow
};
