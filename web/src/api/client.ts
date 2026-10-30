export type User = { id: number; username: string; email: string };

export type Platform = {
  id: string;
  name: string;
  directory: string;
  hash_mode: string;
  rename_policy: string;
  screenscraper_id?: number;
  scrape_as?: string;
  rom_count?: number;
  pending_en?: number;
  missing_covers?: number;
};

export type Stats = {
  platforms: number;
  platforms_with_roms: number;
  games: number;
  roms: number;
  missing: number;
  unidentified: number;
  locked_titles: number;
  missing_covers: number;
  pending_en: number;
  missing_description: number;
};

export type MediaAsset = {
  kind: string;
  path: string;
  source: string;
};

export type Game = {
  id: number;
  platform_id: string;
  platform_name: string;
  title_zh: string;
  title_en: string;
  description: string;
  developer: string;
  release_year: number;
  title_zh_locked: boolean;
  description_locked: boolean;
  scrape_id: string;
  rom_path: string;
  filename: string;
  missing: boolean;
  sha1: string;
  cover_path: string;
  media?: MediaAsset[];
};

export type ScrapeCandidate = {
  id: string;
  provider: string;
  label: string;
  title_zh: string;
  title_en: string;
  description: string;
  developer: string;
  release_year?: number;
  medias: Partial<Record<"box" | "screenshot" | "title" | "logo", string>>;
};

export type ScrapeSide = "left" | "right";

export type ScrapeApplyChoices = {
  title_zh: ScrapeSide;
  title_en: ScrapeSide;
  description: ScrapeSide;
  developer: ScrapeSide;
  release_year: ScrapeSide;
  box: ScrapeSide;
  screenshot: ScrapeSide;
  title: ScrapeSide;
  logo: ScrapeSide;
};

export type MediaKind = "box" | "screenshot" | "title" | "logo" | "video";

export type ScrapeMediaImage = {
  url: string;
  thumb?: string;
  width?: number;
  height?: number;
  style?: string;
  source?: string;
};

export type ScrapeMediaBrowse = {
  provider: string;
  kind: string;
  game_id?: string;
  game_label?: string;
  items: ScrapeMediaImage[];
};

/** Which scrape sources can supply a given media kind. */
export function providerSupportsKind(provider: string, kind: MediaKind): boolean {
  if (kind === "video") return false;
  const p = provider.toLowerCase();
  switch (p) {
    case "screenscraper":
    case "ss":
      return true;
    case "steamgriddb":
    case "sgdb":
      return kind === "box" || kind === "title" || kind === "logo";
    case "igdb":
      return kind === "box" || kind === "title" || kind === "screenshot";
    default:
      return false;
  }
}

export type Job = {
  id: number;
  type: string;
  status: string;
  progress: number;
  message: string;
  report?: unknown;
};

export type ProviderPublic = {
  configured: boolean;
  user_set?: boolean;
  dev_id?: string;
  user_ss?: string;
  softname?: string;
  client_id?: string;
  api_key_set?: boolean;
};

export type ScrapeStatus = {
  screenscraper_configured: boolean;
  screenscraper_user_set: boolean;
  igdb_configured: boolean;
  steamgriddb_configured: boolean;
  any_configured: boolean;
  missing_covers: number;
};

export type LLMPublic = {
  configured: boolean;
  base_url?: string;
  model?: string;
  api_key_set?: boolean;
};

export type Settings = {
  paths: Record<string, string>;
  providers?: {
    screenscraper: ProviderPublic;
    igdb: ProviderPublic;
    steamgriddb: ProviderPublic;
    llm?: LLMPublic;
  };
  screenscraper_configured: boolean;
  screenscraper_user_set: boolean;
  igdb_configured: boolean;
  steamgriddb_configured: boolean;
  any_scrape_configured: boolean;
  llm_configured?: boolean;
};

export type TranslateReport = {
  platform_id: string;
  platform: string;
  attempted: number;
  updated: number;
  skipped: number;
  failed?: ScrapeFailure[];
};

export type ScrapeFailure = {
  game_id: number;
  filename: string;
  kind: string;
  message: string;
};

export type ScrapeReport = {
  mode: string;
  attempted: number;
  updated: number;
  media: number;
  cached: number;
  not_found: number;
  skipped: number;
  sources?: Record<string, number>;
  failed?: ScrapeFailure[];
};

export type ScrapeFields = {
  title_zh: boolean;
  title_en: boolean;
  description: boolean;
  developer: boolean;
  box: boolean;
  screenshot: boolean;
  title: boolean;
  logo: boolean;
};

export type ScrapeWriteMode = "update" | "overwrite";

export type ScrapeRequest = {
  platform_id?: string;
  only_missing?: boolean;
  write_mode?: ScrapeWriteMode;
  game_id?: number;
  fields?: Partial<ScrapeFields>;
};

export const defaultScrapeFields = (): ScrapeFields => ({
  title_zh: false,
  title_en: true,
  description: true,
  developer: true,
  box: true,
  screenshot: true,
  title: true,
  logo: true,
});

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers || {}),
    },
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error((data as { message?: string }).message || res.statusText);
  }
  return data as T;
}

export function mediaURL(coverPath: string): string {
  if (!coverPath) return "";
  return `/api/v1/media/${coverPath.split("/").map(encodeURIComponent).join("/")}`;
}

export const api = {
  authStatus: () => request<{ has_user: boolean }>("/api/v1/auth/status"),
  me: () => fetch("/api/v1/auth/me").then(async (r) => (r.ok ? ((await r.json()) as User) : null)),
  register: (body: object) => request<User>("/api/v1/auth/register", { method: "POST", body: JSON.stringify(body) }),
  login: (body: object) => request<User>("/api/v1/auth/login", { method: "POST", body: JSON.stringify(body) }),
  logout: () => request<{ status: string }>("/api/v1/auth/logout", { method: "POST" }),
  platforms: () => request<{ items: Platform[] }>("/api/v1/platforms"),
  stats: () => request<Stats>("/api/v1/library/stats"),
  games: (params: { platform?: string; q?: string; limit?: number; sort?: string }) => {
    const q = new URLSearchParams();
    if (params.platform) q.set("platform", params.platform);
    if (params.q) q.set("q", params.q);
    if (params.sort) q.set("sort", params.sort);
    q.set("limit", String(params.limit ?? 200));
    return request<{ items: Game[] }>(`/api/v1/games?${q}`);
  },
  game: (id: number) => request<Game>(`/api/v1/games/${id}`),
  updateGame: (id: number, body: object) =>
    request<Game>(`/api/v1/games/${id}`, { method: "PATCH", body: JSON.stringify(body) }),
  uploadGameMedia: async (id: number, kind: MediaKind, file: File) => {
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`/api/v1/games/${id}/media/${kind}`, { method: "POST", body: form });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error((data as { message?: string }).message || res.statusText);
    }
    return data as { media: MediaAsset; game: Game };
  },
  scrapeSearch: (id: number, body: { provider: string; query: string }) =>
    request<{ items: ScrapeCandidate[]; provider: string; query: string }>(
      `/api/v1/games/${id}/scrape/search`,
      { method: "POST", body: JSON.stringify(body) },
    ),
  scrapePreview: (id: number, body: { provider: string; candidate_id: string; query?: string }) =>
    request<{ candidate: ScrapeCandidate; choices: ScrapeApplyChoices; left: Game }>(
      `/api/v1/games/${id}/scrape/preview`,
      { method: "POST", body: JSON.stringify(body) },
    ),
  scrapeApply: (id: number, body: { candidate: ScrapeCandidate; choices: ScrapeApplyChoices }) =>
    request<Game>(`/api/v1/games/${id}/scrape/apply`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  scrapeMediaBrowse: (id: number, body: { provider: string; kind: MediaKind; query?: string }) =>
    request<ScrapeMediaBrowse>(`/api/v1/games/${id}/scrape/media/browse`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  scrapeMediaPick: (id: number, body: { provider: string; kind: MediaKind; url: string }) =>
    request<Game>(`/api/v1/games/${id}/scrape/media/pick`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  settings: () => request<Settings>("/api/v1/settings"),
  updateScreenScraper: (body: object) =>
    request<Partial<ScrapeStatus>>("/api/v1/settings/screenscraper", { method: "PATCH", body: JSON.stringify(body) }),
  updateIGDB: (body: object) =>
    request<Partial<ScrapeStatus>>("/api/v1/settings/igdb", { method: "PATCH", body: JSON.stringify(body) }),
  updateSteamGridDB: (body: object) =>
    request<Partial<ScrapeStatus>>("/api/v1/settings/steamgriddb", { method: "PATCH", body: JSON.stringify(body) }),
  updateLLM: (body: object) =>
    request<LLMPublic>("/api/v1/settings/llm", { method: "PATCH", body: JSON.stringify(body) }),
  llmModels: (params?: { base_url?: string; api_key?: string }) => {
    const q = new URLSearchParams();
    if (params?.base_url) q.set("base_url", params.base_url);
    if (params?.api_key) q.set("api_key", params.api_key);
    const qs = q.toString();
    return request<{ items: { id: string }[]; model?: string }>(`/api/v1/llm/models${qs ? `?${qs}` : ""}`);
  },
  testLLM: (body?: object) =>
    request<{ ok: boolean; message: string }>("/api/v1/llm/test", {
      method: "POST",
      body: JSON.stringify(body || {}),
    }),
  scrapeStatus: () => request<ScrapeStatus>("/api/v1/scrape/status"),
  testScrapeProvider: (provider: string) =>
    request<{ ok: boolean; provider: string; message: string }>("/api/v1/scrape/test", {
      method: "POST",
      body: JSON.stringify({ provider }),
    }),
  scan: () => request<Job>("/api/v1/jobs/scan", { method: "POST" }),
  importPegasus: () => request<Job>("/api/v1/jobs/import-pegasus", { method: "POST" }),
  scrapeMissing: () => request<Job>("/api/v1/jobs/scrape-missing", { method: "POST" }),
  scrapePlatform: (
    platformId: string,
    opts?: { only_missing?: boolean; write_mode?: ScrapeWriteMode; fields?: Partial<ScrapeFields> },
  ) =>
    request<Job>("/api/v1/jobs/scrape-platform", {
      method: "POST",
      body: JSON.stringify({ platform_id: platformId, ...opts }),
    }),
  scrapeOne: (gameId: number, fields?: Partial<ScrapeFields>, writeMode?: ScrapeWriteMode) =>
    request<Job>("/api/v1/jobs/scrape-one", {
      method: "POST",
      body: JSON.stringify({ game_id: gameId, fields, write_mode: writeMode }),
    }),
  scrape: (body: ScrapeRequest) =>
    request<Job>("/api/v1/jobs/scrape", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  translatePlatform: (platformId: string) =>
    request<Job>("/api/v1/jobs/translate-platform", {
      method: "POST",
      body: JSON.stringify({ platform_id: platformId }),
    }),
  job: (id: number) => request<Job>(`/api/v1/jobs/${id}`),
  jobs: () => request<{ items: Job[] }>("/api/v1/jobs"),
};

export async function waitJob(id: number, onTick?: (j: Job) => void): Promise<Job> {
  for (;;) {
    const j = await api.job(id);
    onTick?.(j);
    if (j.status === "succeeded" || j.status === "failed") return j;
    await new Promise((r) => setTimeout(r, 600));
  }
}
