// UI qualifier for the explicit Facebook Page selection. Go still validates ownership,
// input syntax, permissions and CAS. Never silently repoint a fully qualified source.
export function pageSourceInput(input: string, page: string): string | null {
  const value = input.trim();
  if (!/^[0-9]{1,40}$/.test(page)) return null;
  if (/^[0-9]{1,39}$/.test(value)) return `${page}_${value}`;
  const post = /^([0-9]{1,40})_([0-9]{1,39})$/.exec(value);
  if (post) return post[1] === page ? value : null;
  try {
    const url = new URL(value.includes("://") ? value : `https://${value}`);
    if (url.protocol !== "https:" || url.username || url.password || url.port ||
      !["facebook.com", "www.facebook.com", "m.facebook.com", "web.facebook.com"].includes(url.hostname)) return null;
    const parts = /^\/([0-9]{1,40})\/(?:posts|videos)\/([0-9]{1,39})\/?$/.exec(url.pathname);
    return parts?.[1] === page ? value : null;
  } catch { return null; }
}
