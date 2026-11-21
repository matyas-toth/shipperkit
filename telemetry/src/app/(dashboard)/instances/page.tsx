import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { IconChevronLeft, IconChevronRight } from "@tabler/icons-react";
import { requireAdmin } from "@/lib/auth";
import { getInstances, PAGE_SIZE, type InstanceFilter } from "@/lib/metrics";
import { countryName, formatAgo, formatDate, formatNumber, label } from "@/lib/format";
import { PageHeader } from "@/components/PageHeader";
import { Segmented, SegmentedView } from "@/components/Segmented";
import { TableSkeleton } from "../skeletons";

export const metadata: Metadata = { title: "Instances" };

const FILTERS = [
  { value: "active", label: "Active" },
  { value: "inactive", label: "Inactive" },
  { value: "all", label: "All" },
];

export default function InstancesPage({ searchParams }: PageProps<"/instances">) {
  return (
    <>
      <PageHeader title="Instances">
        <Suspense fallback={<SegmentedView param="status" options={FILTERS} current="active" />}>
          <Segmented param="status" options={FILTERS} fallback="active" />
        </Suspense>
      </PageHeader>
      <Suspense fallback={<TableSkeleton />}>
        <InstanceTable searchParams={searchParams} />
      </Suspense>
    </>
  );
}

const th = "h-[38px] px-4 text-left text-[12px] font-[520] text-muted whitespace-nowrap";
const td = "h-12 px-4 whitespace-nowrap";

async function InstanceTable({ searchParams }: { searchParams: PageProps<"/instances">["searchParams"] }) {
  const params = await searchParams;
  const status: InstanceFilter = params.status === "inactive" || params.status === "all" ? params.status : "active";
  const page = Math.max(1, Number.parseInt(String(params.page ?? "1"), 10) || 1);
  await requireAdmin();
  const { rows, count, generatedAt: now } = await getInstances(status, page);
  const pages = Math.max(1, Math.ceil(count / PAGE_SIZE));
  const href = (target: number) => `?${new URLSearchParams({ status, page: String(target) })}`;

  return (
    <div className="grid gap-3">
      <div className="overflow-x-auto rounded-card border border-border bg-surface">
        <table className="w-full min-w-[860px] border-collapse text-[13px]">
          <thead className="border-b border-border">
            <tr>
              <th className={th}>Instance</th>
              <th className={th}>Version</th>
              <th className={th}>Host</th>
              <th className={th}>Install</th>
              <th className={`${th} text-right`}>Apps</th>
              <th className={th}>Country</th>
              <th className={`${th} text-right`}>Reports</th>
              <th className={th}>First seen</th>
              <th className={th}>Last seen</th>
            </tr>
          </thead>
          <tbody>
            {rows.map(row => (
              <tr key={row.id} className="border-b border-[#262626] transition-colors last:border-0 hover:bg-layer-02/60">
                <td className={td}>
                  <span className="flex items-center gap-2.5">
                    <span className={`size-2 rounded-full ${row.active ? "bg-success shadow-[0_0_0_3px_#7fdaa61f]" : "bg-[#5b5c5a]"}`} aria-label={row.active ? "Active" : "Inactive"} role="img" />
                    <span className="font-mono text-[12px] text-text" title={row.id}>
                      {row.id.slice(0, 12)}
                    </span>
                  </span>
                </td>
                <td className={`${td} font-mono text-[12px] text-text`}>{row.version}</td>
                <td className={`${td} text-secondary`}>
                  {row.hostOs} <span className="text-muted">{label(row.arch)}</span>
                </td>
                <td className={`${td} text-secondary`}>
                  {row.install}
                  {row.channel === "Nightly" && <span className="ml-2 rounded-[5px] bg-[#c985001f] px-1.5 py-0.5 text-[11px] font-[560] text-warning">Nightly</span>}
                </td>
                <td className={`${td} tabular text-right text-text`}>{formatNumber(row.applications)}</td>
                <td className={`${td} text-secondary`}>{row.country ? countryName(row.country) : "–"}</td>
                <td className={`${td} tabular text-right text-secondary`}>{formatNumber(row.reports)}</td>
                <td className={`${td} tabular text-secondary`}>{formatDate(row.firstSeenAt)}</td>
                <td className={`${td} tabular ${row.active ? "text-text" : "text-muted"}`} title={new Date(row.lastSeenAt).toISOString()}>
                  {formatAgo(row.lastSeenAt, now)}
                </td>
              </tr>
            ))}
            {!rows.length && (
              <tr>
                <td colSpan={9} className="h-40 text-center text-muted">
                  No instances
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      {pages > 1 && (
        <nav className="flex items-center justify-end gap-2 text-[13px] text-secondary" aria-label="Pages">
          <span className="tabular mr-2">
            {page} of {pages}
          </span>
          <PageLink href={href(page - 1)} disabled={page <= 1} label="Previous page">
            <IconChevronLeft size={16} />
          </PageLink>
          <PageLink href={href(page + 1)} disabled={page >= pages} label="Next page">
            <IconChevronRight size={16} />
          </PageLink>
        </nav>
      )}
    </div>
  );
}

function PageLink({ href, disabled, label, children }: { href: string; disabled: boolean; label: string; children: React.ReactNode }) {
  const className = "grid size-8 place-items-center rounded-control border border-border bg-surface";
  if (disabled) return <span className={`${className} opacity-40`} aria-disabled="true">{children}</span>;
  return (
    <Link href={href} aria-label={label} className={`${className} text-text transition-colors hover:bg-layer-02`} scroll={false}>
      {children}
    </Link>
  );
}
