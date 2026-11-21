import { Suspense } from "react";
import { IconBroadcast } from "@tabler/icons-react";
import { requireAdmin } from "@/lib/auth";
import { getOverview, RANGES, type Overview, type RangeKey } from "@/lib/metrics";
import { countryName, formatCompact, formatDuration, formatNumber, formatPercent, label } from "@/lib/format";
import { AQUA, BLUE, GREEN, MAGENTA, ORANGE, RED, VIOLET, YELLOW } from "@/lib/colors";
import { BarList, Card, Facts, SectionTitle, SplitBar, Stat } from "@/components/Blocks";
import { Columns, DailyBars, TrendChart, VersionChart } from "@/components/Charts";
import { PageHeader } from "@/components/PageHeader";
import { Segmented, SegmentedView } from "@/components/Segmented";
import { OverviewSkeleton } from "./skeletons";

const RANGE_OPTIONS = [
  { value: "30d", label: "30 days" },
  { value: "90d", label: "90 days" },
  { value: "1y", label: "1 year" },
];

export default function OverviewPage({ searchParams }: PageProps<"/">) {
  return (
    <>
      <PageHeader title="Overview">
        <Suspense fallback={<SegmentedView param="range" options={RANGE_OPTIONS} current="30d" />}>
          <Segmented param="range" options={RANGE_OPTIONS} fallback="30d" />
        </Suspense>
      </PageHeader>
      <Suspense fallback={<OverviewSkeleton />}>
        <OverviewData searchParams={searchParams} />
      </Suspense>
    </>
  );
}

async function OverviewData({ searchParams }: { searchParams: PageProps<"/">["searchParams"] }) {
  const { range } = await searchParams;
  const key: RangeKey = typeof range === "string" && range in RANGES ? (range as RangeKey) : "30d";
  await requireAdmin();
  const data = await getOverview(key);
  return data.kpis.total ? <OverviewView data={data} /> : <NoReports />;
}

function NoReports() {
  return (
    <div className="grid min-h-[360px] place-items-center rounded-card border border-border bg-surface p-8 text-center">
      <div className="grid justify-items-center gap-3">
        <span className="grid size-11 place-items-center rounded-[10px] bg-layer-02 text-secondary">
          <IconBroadcast size={22} stroke={1.6} />
        </span>
        <h2 className="text-[18px] font-[600]">No reports yet</h2>
        <code className="rounded-[6px] border border-border bg-deep px-2.5 py-1 font-mono text-[12px] text-secondary">POST /api/v1/ingest</code>
      </div>
    </div>
  );
}

function OverviewView({ data }: { data: Overview }) {
  const { kpis, usage } = data;
  const active = kpis.active;
  const delta = kpis.activeBefore ? (kpis.active - kpis.activeBefore) / kpis.activeBefore : null;
  const totals = (items: { value: number }[]) => items.reduce((sum, item) => sum + item.value, 0);
  const newInRange = data.days.reduce((sum, day) => sum + day.new, 0);
  const upgradesInRange = data.days.reduce((sum, day) => sum + day.upgrades, 0);

  return (
    <div className="grid gap-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        <Stat label="Active" value={formatNumber(active)} delta={delta} color={BLUE} />
        <Stat label="All time" value={formatNumber(kpis.total)} color={VIOLET} />
        <Stat label="New" value={formatNumber(kpis.new)} color={AQUA} />
        <Stat label="Inactive" value={formatNumber(kpis.inactive)} color="#6f706c" />
        <Stat label="Week 1 retention" value={kpis.retention == null ? "–" : formatPercent(kpis.retention)} color={YELLOW} />
        <Stat label="Applications" value={formatCompact(kpis.applications)} color={MAGENTA} />
      </div>

      <Card title="Active instances" figure={formatNumber(active)}>
        <TrendChart days={data.days} dataKey="active" name="Active instances" color={BLUE} height={260} />
      </Card>

      <div className="grid gap-4 md:grid-cols-2">
        <Card title="New instances" figure={formatNumber(newInRange)}>
          <DailyBars days={data.days} dataKey="new" name="New instances" color={AQUA} />
        </Card>
        <Card title="Upgrades" figure={formatNumber(upgradesInRange)}>
          <DailyBars days={data.days} dataKey="upgrades" name="Upgrades" color={VIOLET} />
        </Card>
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <Card title="Version adoption" className="lg:col-span-2">
          <VersionChart days={data.days} keys={data.versionKeys} />
        </Card>
        <Card title="Versions" figure={formatNumber(data.versions.length)}>
          <BarList items={data.versions.slice(0, 9)} total={active} color={BLUE} />
        </Card>
      </div>

      <SectionTitle>Platform</SectionTitle>
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card title="Host OS">
          <BarList items={data.hostOs} total={active} color={AQUA} />
        </Card>
        <Card title="Docker">
          <BarList items={data.docker} total={active} color={ORANGE} />
        </Card>
        <Card title="Countries">
          <BarList items={data.countries} total={active} color={MAGENTA} labelFor={countryName} />
        </Card>
        <Card title="Architecture">
          <SplitBar items={data.arch} labelFor={label} />
        </Card>
        <Card title="Install">
          <SplitBar items={data.install} />
        </Card>
        <Card title="Update channel">
          <SplitBar items={data.channel} />
        </Card>
        <Card title="CPU cores">
          <Columns items={data.cpus} name="Instances" color={YELLOW} />
        </Card>
        <Card title="Memory">
          <Columns items={data.memory} name="Instances" color={VIOLET} />
        </Card>
        <Card title="Applications per instance">
          <Columns items={data.appsPerInstance} name="Instances" color={BLUE} />
        </Card>
      </div>

      <SectionTitle>Usage</SectionTitle>
      <Facts
        items={[
          { label: "Apps per instance", value: active ? (usage.applications / active).toFixed(1) : "–" },
          { label: "Services", value: formatCompact(usage.services) },
          { label: "Domains", value: formatCompact(usage.domains) },
          { label: "Users", value: formatCompact(usage.users) },
          { label: "Deploys in 24h", value: formatCompact(usage.deployments) },
          { label: "Deploy failure rate", value: usage.deployments ? formatPercent(usage.failed / usage.deployments) : "–" },
          { label: "Rollbacks in 24h", value: formatCompact(usage.rollbacks) },
          { label: "Median deploy", value: usage.medianDeploy == null ? "–" : formatDuration(usage.medianDeploy) },
          { label: "With backups", value: active ? formatPercent(usage.backups / active) : "–" },
          { label: "With tasks", value: active ? formatPercent(usage.tasks / active) : "–" },
        ]}
      />
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card title="Build methods" figure={formatCompact(totals(data.methods))}>
          <BarList items={data.methods} total={totals(data.methods)} color={VIOLET} labelFor={label} />
        </Card>
        <Card title="Sources" figure={formatCompact(totals(data.sources))}>
          <BarList items={data.sources} total={totals(data.sources)} color={BLUE} labelFor={label} />
        </Card>
        <Card title="Databases" figure={formatCompact(totals(data.databases))}>
          <BarList items={data.databases} total={totals(data.databases)} color={AQUA} labelFor={label} />
        </Card>
        <Card title="Catalog services" figure={formatCompact(totals(data.catalog))}>
          <BarList items={data.catalog} total={totals(data.catalog)} color={MAGENTA} labelFor={label} />
        </Card>
        <Card title="Connections" figure={formatCompact(totals(data.connections))}>
          <BarList items={data.connections} total={totals(data.connections)} color={ORANGE} labelFor={label} />
        </Card>
        <Card title="Instance lifetime">
          <Columns items={data.lifetime} name="Instances" color={RED} />
        </Card>
      </div>
      <Card title="Feature adoption">
        <div className="grid gap-x-6 gap-y-1 md:grid-cols-2">
          <BarList items={data.features.slice(0, Math.ceil(data.features.length / 2))} total={active} color={GREEN} format="share" labelFor={label} />
          {data.features.length > 1 && <BarList items={data.features.slice(Math.ceil(data.features.length / 2))} total={active} color={GREEN} format="share" labelFor={label} />}
        </div>
      </Card>
    </div>
  );
}
