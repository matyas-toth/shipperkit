export function PageHeader({ title, children }: { title: string; children?: React.ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
      <h1 className="text-[30px] leading-9 font-[620] tracking-[-0.8px]">{title}</h1>
      {children}
    </div>
  );
}
