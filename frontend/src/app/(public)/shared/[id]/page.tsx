'use client';

import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { File } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { FileIconComponent } from '@/components/file-icon';
import { Download, AlertCircle, Loader2, LogIn } from 'lucide-react';
import { format } from 'date-fns';

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? '';

async function fetchPublicFile(id: string, token?: string | null): Promise<File> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(`${API_URL}/api/public/files/${id}`, { headers });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'Failed to load file');
  return data.data;
}

async function fetchPublicChildren(id: string, token?: string | null): Promise<File[]> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(`${API_URL}/api/public/files/${id}/children`, { headers });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'Failed to load files');
  return data.data;
}

function formatBytes(bytes: number, decimals = 2) {
  if (!+bytes) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

export default function PublicSharedPage() {
  const params = useParams();
  const id = params.id as string;
  const [file, setFile] = useState<File | null>(null);
  const [children, setChildren] = useState<File[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [requiresLogin, setRequiresLogin] = useState(false);

  useEffect(() => {
    const load = async () => {
      setIsLoading(true);
      setError(null);
      setRequiresLogin(false);
      const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
      try {
        const f = await fetchPublicFile(id, token);
        setFile(f);
        if (f.isDirectory) {
          const c = await fetchPublicChildren(id, token);
          setChildren(c);
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to load';
        if (message.toLowerCase().includes('login required')) {
          setRequiresLogin(true);
        }
        setError(message);
      } finally {
        setIsLoading(false);
      }
    };
    load();
  }, [id]);

  const handleDownload = (fileId: string, fileName: string) => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
    const url = `${API_URL}/api/public/files/${fileId}/download`;
    const a = document.createElement('a');
    if (token) {
      fetch(url, { headers: { Authorization: `Bearer ${token}` } })
        .then(r => r.blob())
        .then(blob => {
          a.href = URL.createObjectURL(blob);
          a.download = fileName;
          a.click();
          URL.revokeObjectURL(a.href);
        });
    } else {
      a.href = url;
      a.download = fileName;
      a.click();
    }
  };

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-50">
        <Loader2 className="h-8 w-8 animate-spin text-slate-400" />
      </div>
    );
  }

  if (requiresLogin) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-50 p-4">
        <div className="max-w-md text-center space-y-4">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-amber-100">
            <LogIn className="h-8 w-8 text-amber-600" />
          </div>
          <h2 className="text-xl font-semibold">Login Required</h2>
          <p className="text-muted-foreground">
            You need to be logged in to access this shared file.
          </p>
          <Button asChild>
            <Link href="/login">Log in</Link>
          </Button>
        </div>
      </div>
    );
  }

  if (error || !file) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-50 p-4">
        <div className="max-w-md text-center space-y-4">
          <AlertCircle className="mx-auto h-12 w-12 text-slate-400" />
          <h2 className="text-xl font-semibold">File Not Found</h2>
          <p className="text-muted-foreground">{error || 'This shared link is invalid or has expired.'}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b bg-white px-6 py-4">
        <div className="mx-auto flex max-w-4xl items-center justify-between">
          <Link href="/" className="text-lg font-bold text-slate-900">DocShare</Link>
          <span className="text-xs text-muted-foreground">Shared file</span>
        </div>
      </header>

      <main className="mx-auto max-w-4xl p-6 space-y-6">
        <div className="flex items-center gap-4">
          <div className="rounded-xl border bg-white p-4 shadow-sm">
            <FileIconComponent mimeType={file.mimeType} isDirectory={file.isDirectory} className="h-12 w-12 text-blue-600" />
          </div>
          <div className="min-w-0 flex-1">
            <h1 className="text-2xl font-bold truncate">{file.name}</h1>
            <p className="text-sm text-muted-foreground">
              {file.isDirectory ? 'Folder' : `${formatBytes(file.size)} · ${file.mimeType}`}
              {file.owner && ` · Shared by ${file.owner.firstName} ${file.owner.lastName}`}
            </p>
          </div>
          {!file.isDirectory && (
            <Button onClick={() => handleDownload(file.id, file.name)}>
              <Download className="mr-2 h-4 w-4" />
              Download
            </Button>
          )}
        </div>

        {file.isDirectory && (
          <div className="rounded-lg border bg-white">
            {children.length === 0 ? (
              <div className="p-8 text-center text-muted-foreground">This folder is empty.</div>
            ) : (
              <div className="divide-y">
                {children.map((child) => (
                  <div key={child.id} className="flex items-center justify-between px-4 py-3 hover:bg-slate-50 transition-colors">
                    <Link href={`/shared/${child.id}`} className="flex items-center gap-3 min-w-0 flex-1">
                      <FileIconComponent mimeType={child.mimeType} isDirectory={child.isDirectory} className="h-5 w-5 shrink-0 text-blue-600" />
                      <div className="min-w-0">
                        <p className="truncate font-medium text-sm">{child.name}</p>
                        <p className="text-xs text-muted-foreground">
                          {child.isDirectory ? 'Folder' : formatBytes(child.size)}
                          {child.updatedAt && ` · ${format(new Date(child.updatedAt), 'MMM d, yyyy')}`}
                        </p>
                      </div>
                    </Link>
                    {!child.isDirectory && (
                      <Button variant="ghost" size="icon" className="shrink-0" onClick={() => handleDownload(child.id, child.name)}>
                        <Download className="h-4 w-4" />
                      </Button>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
