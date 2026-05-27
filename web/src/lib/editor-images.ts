import { toast } from 'sonner';

export const MAX_IMAGE_BYTES = 2 * 1024 * 1024;

const ALLOWED_TYPES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/svg+xml']);

export async function fileToDataURI(file: File): Promise<string | null> {
  if (!ALLOWED_TYPES.has(file.type)) {
    toast.error(`Unsupported image type: ${file.type || 'unknown'}`);
    return null;
  }
  if (file.size > MAX_IMAGE_BYTES) {
    toast.error(`Image is ${(file.size / 1024 / 1024).toFixed(1)} MiB — limit is 2 MiB. Upload as a separate file and link to it instead.`);
    return null;
  }
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : null);
    reader.onerror = () => reject(reader.error ?? new Error('failed to read image'));
    reader.readAsDataURL(file);
  });
}

export function isImageFile(file: File): boolean {
  return file.type.startsWith('image/');
}
