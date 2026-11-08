/* Gets a chosen picture ready for the website, in the browser, before it is uploaded. Anything the browser can open works (a WebP saved
   from a website, even when it is named .png), and it comes out as a JPEG or PNG of a sensible size.
   logo: the empty white or transparent margin is cut away, so logos all fill their card the same. ratio: cut evenly from the edges to that
   width / height, so a replacement has the same shape as the picture it replaces. */
const startsWith = (head: Uint8Array, sig: number[]) => sig.every((b, i) => head[i] === b);

function blank(px: Uint8ClampedArray, i: number) { return px[i + 3] < 16 || (px[i] > 244 && px[i + 1] > 244 && px[i + 2] > 244); }

function trimmed(c: HTMLCanvasElement): HTMLCanvasElement {
  const ctx = c.getContext('2d')!;
  const px = ctx.getImageData(0, 0, c.width, c.height).data;
  let top = c.height, left = c.width, right = -1, bottom = -1;
  for (let y = 0; y < c.height; y++) for (let x = 0; x < c.width; x++) {
    if (blank(px, (y * c.width + x) * 4)) continue;
    top = Math.min(top, y); bottom = Math.max(bottom, y); left = Math.min(left, x); right = Math.max(right, x);
  }
  if (right < 0) return c; // nothing but blank: leave it for the person to see
  const out = document.createElement('canvas');
  out.width = right - left + 1; out.height = bottom - top + 1;
  out.getContext('2d')!.drawImage(c, left, top, out.width, out.height, 0, 0, out.width, out.height);
  return out;
}

export async function fitImage(file: File, { logo = false, ratio }: { logo?: boolean; ratio?: number } = {}): Promise<Blob> {
  const unreadable = () => new Error(`"${file.name}" could not be opened as a picture. Save it as a JPEG or PNG and try again.`);
  const maxSide = logo ? 1200 : 2400;
  const url = URL.createObjectURL(file);
  try {
    const img = new Image();
    img.src = url;
    await img.decode().catch(() => { throw unreadable(); });
    const w = img.naturalWidth, h = img.naturalHeight;
    if (!w || !h) throw unreadable();
    // Already a real JPEG or PNG that needs no change: send it as it is, so a photo is not compressed twice.
    const head = new Uint8Array(await file.slice(0, 8).arrayBuffer());
    const plain = startsWith(head, [0xff, 0xd8, 0xff]) || startsWith(head, [0x89, 0x50, 0x4e, 0x47]);
    if (plain && !logo && !ratio && Math.max(w, h) <= maxSide && file.size <= 5 * 1024 * 1024) return file;

    let sw = w, sh = h;
    if (ratio) { if (w / h > ratio) sw = Math.round(h * ratio); else sh = Math.round(w / ratio); }
    const scale = Math.min(1, maxSide / Math.max(sw, sh));
    let canvas = document.createElement('canvas');
    canvas.width = Math.max(1, Math.round(sw * scale)); canvas.height = Math.max(1, Math.round(sh * scale));
    const ctx = canvas.getContext('2d');
    if (!ctx) throw unreadable();
    if (!logo) { ctx.fillStyle = '#fff'; ctx.fillRect(0, 0, canvas.width, canvas.height); } // a JPEG has no see-through parts
    ctx.drawImage(img, (w - sw) / 2, (h - sh) / 2, sw, sh, 0, 0, canvas.width, canvas.height);
    if (logo) canvas = trimmed(canvas);
    const done = canvas;
    const blob = await new Promise<Blob | null>((resolve) => done.toBlob(resolve, logo ? 'image/png' : 'image/jpeg', 0.88));
    if (!blob) throw unreadable();
    return blob;
  } catch (e) {
    throw e instanceof Error && e.message.includes(file.name) ? e : unreadable();
  } finally {
    URL.revokeObjectURL(url);
  }
}
