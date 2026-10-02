// Strict JSON scanner shared by the buyer BFF (lib/buyer-server.ts), split out of it (G-UI3 legacy ceiling). Self-contained: no
// imports, no state. Used for every inbound command body and for upstream payment answers (allowNull).
// JSON.parse alone drops duplicate keys. This small scanner rejects them and
// null at any depth before a commerce command reaches the private transport.
export function strictJSON(text: string, allowNull = false): unknown {
  let at = 0;
  const space = () => {
    while (/\s/.test(text[at] ?? "") && at < text.length) at++;
  };
  const string = (): string => {
    const start = at++;
    while (at < text.length) {
      if (text[at] === "\\") {
        at += 2;
        continue;
      }
      if (text[at++] === '"')
        return JSON.parse(text.slice(start, at)) as string;
    }
    throw new Error("string");
  };
  const value = (): void => {
    space();
    if (text[at] === "{") {
      at++;
      space();
      const keys = new Set<string>();
      if (text[at] === "}") {
        at++;
        return;
      }
      while (true) {
        if (text[at] !== '"') throw new Error("key");
        const key = string();
        if (keys.has(key)) throw new Error("duplicate");
        keys.add(key);
        space();
        if (text[at++] !== ":") throw new Error("colon");
        value();
        space();
        if (text[at] === "}") {
          at++;
          return;
        }
        if (text[at++] !== ",") throw new Error("comma");
        space();
      }
    }
    if (text[at] === "[") {
      at++;
      space();
      if (text[at] === "]") {
        at++;
        return;
      }
      while (true) {
        value();
        space();
        if (text[at] === "]") {
          at++;
          return;
        }
        if (text[at++] !== ",") throw new Error("comma");
      }
    }
    if (text[at] === '"') {
      string();
      return;
    }
    for (const literal of ["true", "false"])
      if (text.startsWith(literal, at)) {
        at += literal.length;
        return;
      }
    if (text.startsWith("null", at)) {
      if (!allowNull) throw new Error("null");
      at += 4;
      return;
    }
    const number = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/.exec(
      text.slice(at),
    );
    if (!number || !Number.isFinite(Number(number[0])))
      throw new Error("value");
    at += number[0].length;
  };
  value();
  space();
  if (at !== text.length) throw new Error("trailing");
  return JSON.parse(text) as unknown;
}
