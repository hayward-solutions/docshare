import { toast } from 'sonner';

export interface DownloadOptions {
  url: string;
  filename: string;
  token?: string;
  onError?: (error: Error) => void;
}

export async function downloadFile({
  url,
  filename,
  token,
  onError
}: DownloadOptions): Promise<void> {
  try {
    const headers: Record<string, string> = {
      'Accept': '*/*',
    };

    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(url, { headers });

    if (!response.ok) {
      throw new Error(`Download failed: ${response.status} ${response.statusText}`);
    }

    const blob = await response.blob();
    const objectUrl = window.URL.createObjectURL(blob);

    const link = document.createElement('a');
    link.href = objectUrl;
    link.download = filename;
    document.body.appendChild(link);

    link.click();

    window.URL.revokeObjectURL(objectUrl);
    document.body.removeChild(link);
  } catch (error) {
    const err = error instanceof Error ? error : new Error('Unknown error');
    if (onError) {
      onError(err);
    } else {
      toast.error(`Failed to download ${filename}`);
    }
    throw err;
  }
}