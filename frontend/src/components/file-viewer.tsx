'use client';

import { useState, useEffect, useRef } from 'react';
import { File } from '@/lib/types';
import { apiMethods } from '@/lib/api';
import { downloadFile } from '@/lib/download';
import { Button } from '@/components/ui/button';
import { Download, AlertCircle, Loader2 } from 'lucide-react';


const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

interface FileViewerProps {
  file: File;
}

async function fetchPreviewBlob(fileId: string): Promise<{ blobUrl: string; contentType: string }> {
  const res = await apiMethods.get<{ path: string; token: string }>(`/api/files/${fileId}/preview`);
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
  const blobUrlRef = useRef<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadPreview = async () => {
      setIsLoading(true);
      setError(null);
      setBlobUrl(null);
      setContent(null);

      if (blobUrlRef.current) {
        URL.revokeObjectURL(blobUrlRef.current);
        blobUrlRef.current = null;
      }

      try {
        if (isOfficeDoc(file.mimeType)) {
          const res = await apiMethods.get<{ url: string }>(`/api/files/${file.id}/convert-preview`);
          if (cancelled) return;
          if (res.success) {
            setBlobUrl(res.data.url);
          } else {
            setError('Failed to generate preview');
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
    };
  }, [file.id, file.mimeType]);

  const handleDownload = async () => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : undefined;
    await downloadFile({
      url: `${API_URL}/api/files/${file.id}/download`,
      filename: file.name,
      token: token || undefined,
    });
  };

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 p-8 border rounded-lg bg-slate-50">
        <AlertCircle className="h-12 w-12 text-slate-400" />
        <p className="text-slate-600">{error}</p>
        <Button onClick={handleDownload}>
          <Download className="mr-2 h-4 w-4" />
          Download File
        </Button>
      </div>
    );
  }

  if (file.mimeType.startsWith('image/') && blobUrl) {
    return (
      <div className="flex justify-center bg-slate-100 rounded-lg p-4 overflow-hidden">
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
        className="w-full h-[80vh] rounded-lg border bg-white"
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
      <div className="flex items-center justify-center bg-slate-100 rounded-lg p-8">
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
    <div className="flex flex-col items-center justify-center gap-4 p-8 border rounded-lg bg-slate-50">
      <p className="text-slate-600">Preview not available for this file type.</p>
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
    mimeType === 'application/typescript'
  );
}
