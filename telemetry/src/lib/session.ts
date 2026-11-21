// Runs in both Proxy and Server Components, so it sticks to Web Crypto.
export const SESSION_COOKIE = "sk_admin";
export const SESSION_DAYS = 30;

const encoder = new TextEncoder();

function adminKey() {
  const key = process.env.ADMIN_KEY ?? "";
  if (key.length < 48) throw new Error("ADMIN_KEY must be at least 48 characters");
  return key;
}

async function hmac(message: string) {
  const key = await crypto.subtle.importKey("raw", encoder.encode("session:" + adminKey()), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const signature = new Uint8Array(await crypto.subtle.sign("HMAC", key, encoder.encode(message)));
  return Buffer.from(signature).toString("base64url");
}

async function digest(value: string) {
  return new Uint8Array(await crypto.subtle.digest("SHA-256", encoder.encode(value)));
}

function equal(a: Uint8Array, b: Uint8Array) {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a[i] ^ b[i];
  return diff === 0;
}

/** Compares fixed-length digests so timing never reveals key prefixes. */
export async function keyMatches(candidate: string) {
  return equal(await digest(candidate), await digest(adminKey()));
}

export async function createSession() {
  const expires = Date.now() + SESSION_DAYS * 86_400_000;
  return { value: `${expires}.${await hmac(`admin.${expires}`)}`, expires: new Date(expires) };
}

export async function verifySession(value: string | undefined) {
  if (!value) return false;
  const [expires, signature] = value.split(".");
  if (!expires || !signature || !/^\d+$/.test(expires) || Number(expires) < Date.now()) return false;
  return equal(encoder.encode(signature), encoder.encode(await hmac(`admin.${expires}`)));
}
