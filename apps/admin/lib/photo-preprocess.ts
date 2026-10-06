// Purpose: normalize browser photos before upload: EXIF-aware decode, <=2000px, JPEG and <=2MiB.
// Depends on: createImageBitmap/canvas/Blob, fitPhoto and the frozen byte cap; no network call.
// Used by: PM-U draft and server-backed media pickers; immutable result is reused for UNKNOWN retries.
import { MAX_PHOTO_BYTES } from "./image-list-contract";
import { fitPhoto } from "./product-media-model";
/** A local refusal with plain copy; original bytes never reach an upload when normalization fails. */
export class PhotoProblem extends Error {
  constructor(public reason: "type" | "decode" | "size" | "ratio") {
    super(reason);
  }
}
function supported(bytes: Uint8Array) {
  return (
    (bytes[0] === 255 && bytes[1] === 216 && bytes[2] === 255) ||
    (bytes[0] === 137 &&
      bytes[1] === 80 &&
      bytes[2] === 78 &&
      bytes[3] === 71 &&
      bytes[4] === 13 &&
      bytes[5] === 10 &&
      bytes[6] === 26 &&
      bytes[7] === 10) ||
    (String.fromCharCode(...bytes.slice(0, 4)) === "RIFF" &&
      String.fromCharCode(...bytes.slice(8, 12)) === "WEBP")
  );
}
/** Decode orientation before sizing; adapt dimensions at fixed quality if noisy JPEG still exceeds the byte cap. */
export async function preparePhoto(
  file: File,
  role: "main" | "detail" | "sku",
): Promise<{ file: File; width: number; height: number }> {
  if (
    !file.size ||
    !supported(new Uint8Array(await file.slice(0, 16).arrayBuffer()))
  )
    throw new PhotoProblem("type");
  let bitmap: ImageBitmap;
  try {
    bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
  } catch {
    throw new PhotoProblem("decode");
  }
  try {
    if (role === "detail" && bitmap.height > 6 * bitmap.width)
      throw new PhotoProblem("ratio");
    let dimensions = fitPhoto(bitmap.width, bitmap.height);
    const canvas = document.createElement("canvas");
    // Always re-encode: rotated JPEG pixels then have canonical dimensions and no surviving EXIF metadata.
    for (let attempt = 0; attempt < 8; attempt++) {
      canvas.width = dimensions.width;
      canvas.height = dimensions.height;
      const context = canvas.getContext("2d");
      if (!context) throw new PhotoProblem("decode");
      context.fillStyle = "#fff";
      context.fillRect(0, 0, canvas.width, canvas.height);
      context.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
      const blob = await new Promise<Blob | null>((resolve) =>
        canvas.toBlob(resolve, "image/jpeg", 0.85),
      );
      if (!blob) throw new PhotoProblem("decode");
      if (blob.size <= MAX_PHOTO_BYTES)
        return {
          file: new File([blob], file.name.replace(/\.[^.]*$/, "") + ".jpg", {
            type: "image/jpeg",
            lastModified: file.lastModified,
          }),
          width: canvas.width,
          height: canvas.height,
        };
      dimensions = {
        width: Math.max(1, Math.floor(dimensions.width * 0.85)),
        height: Math.max(1, Math.floor(dimensions.height * 0.85)),
      };
    }
    throw new PhotoProblem("size");
  } finally {
    bitmap.close();
  }
}
