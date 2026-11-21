FROM node:24-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json prisma.config.ts ./
COPY prisma ./prisma
RUN npm ci

FROM node:24-alpine AS builder
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1 DATABASE_URL=postgresql://build:build@localhost:5432/build
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

# Migrations run from their own folder so the Prisma CLI stays out of the app bundle.
FROM node:24-alpine AS migrate
WORKDIR /migrate
RUN npm init -y >/dev/null && npm install --no-audit --no-fund prisma@7.10.1
COPY prisma.config.ts ./
COPY prisma ./prisma

FROM node:24-alpine
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
RUN addgroup -S app && adduser -S app -G app
COPY --from=migrate /migrate /migrate
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
USER app
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s CMD wget -qO- http://127.0.0.1:3000/api/health >/dev/null || exit 1
CMD ["sh", "-c", "cd /migrate && npx prisma migrate deploy && cd /app && exec node server.js"]
