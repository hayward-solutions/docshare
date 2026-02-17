'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { File, PreviewJob } from '@/lib/types';
import { apiMethods, previewAPI } from '@/lib/api';
import { downloadFile } from '@/lib/download';
import { Button } from '@/components/ui/button';
import { Download, AlertCircle, Loader2, RefreshCw } from 'lucide-react';


const API_URL = process.env.NEXT_PUBLIC_API_URL ?? '';

interface FileViewerProps {
  file: File;
}

async function fetchPreviewBlob(fileId: string): Promise<{ blobUrl: string; contentType: string }> {
  const res = await apiMethods.get<{ path: string; token: string }>(`/files/${fileId}/preview`);
  if (!res.success) throw new Error('Failed to get preview token');

  const proxyUrl = `${API_URL}${res.data.path}?token=${res.data.token}`;
  const response = await fetch(proxyUrl);
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Preview fetch failed: ${response.status} ${body}`);
  }

  const blob = await response.blob();
  const contentType = response.headers.get('content-type') || '';
  return { blobUrl: URL.createObjectURL(blob), contentType };
}

export function FileViewer({ file }: FileViewerProps) {
  const [blobUrl, setBlobUrl] = useState<string | null>(null);
  const [content, setContent] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [previewJob, setPreviewJob] = useState<PreviewJob | null>(null);
  const blobUrlRef = useRef<string | null>(null);
  const pollingRef = useRef<NodeJS.Timeout | null>(null);

  const pollPreviewStatus = useCallback(async () => {
    if (!isOfficeDoc(file.mimeType)) return;

    try {
      const res = await previewAPI.getStatus(file.id);
      if (res.success && res.data.job) {
        const job = res.data.job as PreviewJob;
        setPreviewJob(job);

        if (job.status === 'completed' && job.thumbnailPath) {
          const { blobUrl: url } = await fetchPreviewBlob(file.id);
          setBlobUrl(url);
          setIsLoading(false);
          if (pollingRef.current) {
            clearInterval(pollingRef.current);
            pollingRef.current = null;
          }
        } else if (job.status === 'failed') {
          setError(job.lastError || 'Preview generation failed');
          setIsLoading(false);
          if (pollingRef.current) {
            clearInterval(pollingRef.current);
            pollingRef.current = null;
          }
        }
      }
    } catch {
      // Silently ignore polling errors
    }
  }, [file.id, file.mimeType]);

  const startPolling = useCallback(() => {
    if (pollingRef.current) return;
    pollingRef.current = setInterval(pollPreviewStatus, 3000);
  }, [pollPreviewStatus]);

  const handleGeneratePreview = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await previewAPI.convert(file.id);
      if (res.success && res.data.job) {
        setPreviewJob(res.data.job as PreviewJob);
        startPolling();
      } else {
        setError('Failed to start preview generation');
        setIsLoading(false);
      }
    } catch {
      setError('Failed to start preview generation');
      setIsLoading(false);
    }
  };

  const handleRetryPreview = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await previewAPI.retry(file.id);
      if (res.success && res.data.job) {
        setPreviewJob(res.data.job as PreviewJob);
        startPolling();
      } else {
        setError('Failed to retry preview generation');
        setIsLoading(false);
      }
    } catch {
      setError('Failed to retry preview generation');
      setIsLoading(false);
    }
  };

  useEffect(() => {
    let cancelled = false;

    const loadPreview = async () => {
      setIsLoading(true);
      setError(null);
      setBlobUrl(null);
      setContent(null);
      setPreviewJob(null);

      if (blobUrlRef.current) {
        URL.revokeObjectURL(blobUrlRef.current);
        blobUrlRef.current = null;
      }

      if (pollingRef.current) {
        clearInterval(pollingRef.current);
        pollingRef.current = null;
      }

      try {
        if (isOfficeDoc(file.mimeType)) {
          const statusRes = await previewAPI.getStatus(file.id);
          if (cancelled) return;

          if (statusRes.success && statusRes.data.job) {
            const job = statusRes.data.job as PreviewJob;
            setPreviewJob(job);

            if (job.status === 'completed' && job.thumbnailPath) {
              const { blobUrl: url } = await fetchPreviewBlob(file.id);
              if (cancelled) { URL.revokeObjectURL(url); return; }
              blobUrlRef.current = url;
              setBlobUrl(url);
              setIsLoading(false);
              return;
            } else if (job.status === 'processing' || job.status === 'pending') {
              startPolling();
              return;
            } else if (job.status === 'failed') {
              setError(job.lastError || 'Preview generation failed');
              setIsLoading(false);
              return;
            }
          }

          const res = await previewAPI.convert(file.id);
          if (cancelled) return;
          if (res.success && res.data.job) {
            setPreviewJob(res.data.job as PreviewJob);
            startPolling();
          } else {
            setError('Failed to generate preview');
            setIsLoading(false);
          }
        } else if (isTextOrCode(file.mimeType)) {
          const { blobUrl: url } = await fetchPreviewBlob(file.id);
          if (cancelled) { URL.revokeObjectURL(url); return; }
          const textResponse = await fetch(url);
          const text = await textResponse.text();
          URL.revokeObjectURL(url);
          if (cancelled) return;
          setContent(text);
        } else {
          const { blobUrl: url } = await fetchPreviewBlob(file.id);
          if (cancelled) { URL.revokeObjectURL(url); return; }
          blobUrlRef.current = url;
          setBlobUrl(url);
        }
      } catch (err) {
        if (!cancelled) {
          console.error('Preview error:', err);
          setError('Preview not available');
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    };

    loadPreview();

    return () => {
      cancelled = true;
      if (blobUrlRef.current) {
        URL.revokeObjectURL(blobUrlRef.current);
        blobUrlRef.current = null;
      }
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
        pollingRef.current = null;
      }
    };
  }, [file.id, file.mimeType, startPolling]);

  const handleDownload = async () => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : undefined;
    await downloadFile({
      url: `${API_URL}/files/${file.id}/download`,
      filename: file.name,
      token: token || undefined,
    });
  };

  if (isLoading) {
    if (isOfficeDoc(file.mimeType) && (previewJob?.status === 'pending' || previewJob?.status === 'processing')) {
      return (
        <div className="flex flex-col items-center justify-center gap-4 p-8 border rounded-lg bg-muted">
          <Loader2 className="h-12 w-12 animate-spin text-primary" />
          <p className="text-muted-foreground">
            {previewJob?.status === 'pending' ? 'Preview queued...' : 'Generating preview...'}
          </p>
          {previewJob && (
            <p className="text-sm text-muted-foreground">
              Attempt {previewJob.attempts} of {previewJob.maxAttempts}
            </p>
          )}
        </div>
      );
    }
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 p-8 border rounded-lg bg-muted">
        <AlertCircle className="h-12 w-12 text-muted-foreground" />
        <p className="text-muted-foreground">{error}</p>
        <div className="flex gap-2">
          {isOfficeDoc(file.mimeType) && previewJob?.status === 'failed' && (
            <Button onClick={handleRetryPreview}>
              <RefreshCw className="mr-2 h-4 w-4" />
              Retry
            </Button>
          )}
          {isOfficeDoc(file.mimeType) && !previewJob && (
            <Button onClick={handleGeneratePreview}>
              <RefreshCw className="mr-2 h-4 w-4" />
              Generate Preview
            </Button>
          )}
          <Button onClick={handleDownload} variant="outline">
            <Download className="mr-2 h-4 w-4" />
            Download File
          </Button>
        </div>
      </div>
    );
  }

  if (file.mimeType.startsWith('image/') && blobUrl) {
    return (
      <div className="flex justify-center bg-muted rounded-lg p-4 overflow-hidden">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={blobUrl}
          alt={file.name}
          className="max-w-full max-h-[80vh] object-contain"
        />
      </div>
    );
  }

  if ((file.mimeType === 'application/pdf' || isOfficeDoc(file.mimeType)) && blobUrl) {
    return (
      <iframe
        src={blobUrl}
        className="w-full h-[80vh] rounded-lg border bg-card"
        title={file.name}
      />
    );
  }

  if (file.mimeType.startsWith('video/') && blobUrl) {
    return (
      <div className="flex justify-center bg-black rounded-lg overflow-hidden">
        <video controls src={blobUrl} className="max-w-full max-h-[80vh]">
          <track kind="captions" />
        </video>
      </div>
    );
  }

  if (file.mimeType.startsWith('audio/') && blobUrl) {
    return (
      <div className="flex items-center justify-center bg-muted rounded-lg p-8">
        <audio controls src={blobUrl} className="w-full max-w-md">
          <track kind="captions" />
        </audio>
      </div>
    );
  }

  if (isTextOrCode(file.mimeType) && content !== null) {
    return (
      <div className="bg-slate-950 text-slate-50 p-4 rounded-lg overflow-auto max-h-[80vh]">
        <pre className="font-mono text-sm whitespace-pre">
          {content}
        </pre>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center gap-4 p-8 border rounded-lg bg-muted">
      <p className="text-muted-foreground">Preview not available for this file type.</p>
      <Button onClick={handleDownload}>
        <Download className="mr-2 h-4 w-4" />
        Download File
      </Button>
    </div>
  );
}

function isOfficeDoc(mimeType: string) {
  return [
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    'application/vnd.openxmlformats-officedocument.presentationml.presentation',
    'application/vnd.oasis.opendocument.text',
    'application/vnd.oasis.opendocument.spreadsheet',
    'application/vnd.oasis.opendocument.presentation'
  ].includes(mimeType);
}

function isTextOrCode(mimeType: string) {
  return (
    mimeType.startsWith('text/') ||
    mimeType === 'application/json' ||
    mimeType === 'application/xml' ||
    mimeType === 'application/javascript' ||
    mimeType === 'application/typescript' ||
    mimeType === 'text/markdown'
  );
}
