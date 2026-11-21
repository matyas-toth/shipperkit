-- CreateTable
CREATE TABLE "instances" (
    "id" TEXT NOT NULL,
    "first_seen_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "last_seen_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "report_count" INTEGER NOT NULL DEFAULT 1,
    "version" TEXT NOT NULL,
    "channel" TEXT NOT NULL,
    "install" TEXT NOT NULL,
    "os" TEXT NOT NULL,
    "arch" TEXT NOT NULL,
    "host_os" TEXT NOT NULL,
    "docker" TEXT NOT NULL,
    "cpus" INTEGER NOT NULL,
    "memory_mb" INTEGER NOT NULL,
    "country" TEXT,
    "applications" INTEGER NOT NULL,
    "latest" JSONB NOT NULL,

    CONSTRAINT "instances_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "reports" (
    "id" BIGSERIAL NOT NULL,
    "instance_id" TEXT NOT NULL,
    "received_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" TEXT NOT NULL,
    "payload" JSONB NOT NULL,

    CONSTRAINT "reports_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "upgrades" (
    "id" BIGSERIAL NOT NULL,
    "instance_id" TEXT NOT NULL,
    "from_version" TEXT NOT NULL,
    "to_version" TEXT NOT NULL,
    "at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "upgrades_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE INDEX "instances_last_seen_at_idx" ON "instances"("last_seen_at");

-- CreateIndex
CREATE INDEX "instances_first_seen_at_idx" ON "instances"("first_seen_at");

-- CreateIndex
CREATE INDEX "reports_received_at_idx" ON "reports"("received_at");

-- CreateIndex
CREATE INDEX "reports_instance_id_received_at_idx" ON "reports"("instance_id", "received_at");

-- CreateIndex
CREATE INDEX "upgrades_at_idx" ON "upgrades"("at");

-- AddForeignKey
ALTER TABLE "reports" ADD CONSTRAINT "reports_instance_id_fkey" FOREIGN KEY ("instance_id") REFERENCES "instances"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "upgrades" ADD CONSTRAINT "upgrades_instance_id_fkey" FOREIGN KEY ("instance_id") REFERENCES "instances"("id") ON DELETE CASCADE ON UPDATE CASCADE;
