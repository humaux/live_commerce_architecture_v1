// Purpose: exercise upload preservation and alpha-safe downsize branches of the actual preprocessor.
// Depends on: Node File/Blob/test, preparePhoto and controlled bitmap/canvas browser API doubles.
// Used by: focused Node regression; real image decode/uploads are independently checked by the PM browser gate.
import test from "node:test";
import assert from "node:assert/strict";
import { preparePhoto } from "../../apps/admin/lib/photo-preprocess.ts";
const png = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
  "base64",
);
const jpeg = Uint8Array.from([255, 216, 255, 224, 0, 2]);
function browser(width: number, height: number, formatSizes: number[] = [100]) {
  let canvases = 0,
    closed = 0,
    fill = 0;
  const formats: string[] = [];
  const originalBitmap = globalThis.createImageBitmap,
    originalDocument = globalThis.document;
  Object.defineProperty(globalThis, "createImageBitmap", {
    configurable: true,
    value: async () => ({
      width,
      height,
      close() {
        closed++;
      },
    }),
  });
  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: {
      createElement() {
        canvases++;
        return {
          width: 0,
          height: 0,
          getContext() {
            return {
              fillRect() {
                fill++;
              },
              drawImage() {},
              clearRect() {},
            };
          },
          toBlob(cb: (b: Blob) => void, type: string) {
            formats.push(type);
            cb(
              new Blob([new Uint8Array(formatSizes.shift() ?? 100)], { type }),
            );
          },
        };
      },
    },
  });
  return {
    get canvases() {
      return canvases;
    },
    get closed() {
      return closed;
    },
    get fill() {
      return fill;
    },
    formats,
    restore() {
      Object.defineProperty(globalThis, "createImageBitmap", {
        configurable: true,
        value: originalBitmap,
      });
      Object.defineProperty(globalThis, "document", {
        configurable: true,
        value: originalDocument,
      });
    },
  };
}
test("a fitting PNG keeps the exact original File, bytes and alpha-capable format", async () => {
  const env = browser(800, 800);
  try {
    const original = new File([png], "logo.png", { type: "image/png" });
    const result = await preparePhoto(original, "main");
    assert.equal(result.file, original);
    assert.deepEqual(
      new Uint8Array(await result.file.arrayBuffer()),
      new Uint8Array(png),
    );
    assert.equal(result.file.type, "image/png");
    assert.equal(env.canvases, 0);
    assert.equal(env.closed, 1);
  } finally {
    env.restore();
  }
});
test("a JPEG exactly at both limits remains original, not a lossy re-encode", async () => {
  const env = browser(2000, 2000);
  try {
    const original = new File(
      [jpeg, new Uint8Array(2 * 1024 * 1024 - jpeg.length)],
      "camera.jpg",
      { type: "image/jpeg" },
    );
    assert.equal((await preparePhoto(original, "main")).file, original);
    assert.equal(env.canvases, 0);
  } finally {
    env.restore();
  }
});
test("an oversized phone JPEG is encoded below the cap after oriented scaling", async () => {
  const env = browser(2400, 3200);
  try {
    const original = new File(
      [jpeg, new Uint8Array(2 * 1024 * 1024)],
      "phone.jpg",
      { type: "image/jpeg" },
    );
    const result = await preparePhoto(original, "detail");
    assert.notEqual(result.file, original);
    assert.deepEqual([result.width, result.height], [1500, 2000]);
    assert.deepEqual(env.formats, ["image/jpeg"]);
    assert.equal(result.file.type, "image/jpeg");
  } finally {
    env.restore();
  }
});
test("a large transparent PNG stays alpha-capable and is never painted with an opaque matte", async () => {
  const env = browser(3000, 2000, [2 * 1024 * 1024 + 1, 100]);
  try {
    const result = await preparePhoto(
      new File([png], "large-logo.png", { type: "image/png" }),
      "main",
    );
    assert.equal(result.file.type, "image/png");
    assert.ok(env.formats.every((type) => type === "image/png"));
    assert.equal(env.fill, 0);
    assert.ok(result.width <= 2000 && result.height <= 2000);
    assert.ok(result.file.size <= 2 * 1024 * 1024);
  } finally {
    env.restore();
  }
});
