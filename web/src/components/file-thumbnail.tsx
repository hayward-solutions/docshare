'use client';

import { useEffect, useRef, useState } from 'react';
import { File } from '@/lib/types';
import { apiMethods } from '@/lib/api';
import { FileIconComponent } from './file-icon';

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? '';

interface FileThumbnailProps {
  file: File;
  className?: string;
  iconClassName?: string;
}

type ThumbKind = 'image' | 'video' | null;

function thumbnailKind(file: File): ThumbKind {
  if (file.isDirectory) return null;
  // Only fetch for images the server has already generated a thumbnail for.
  // Without this gate, pre-feature uploads (no thumbnail_path) would still
  // hit /preview, which falls back to streaming the full original — a grid
  // of 50 phone photos would download hundreds of MB. Files whose job is
  // still pending/failed also fall through here and just show the icon.
  if (file.mimeType.startsWith('image/') && file.thumbnailPath) return 'image';
  if (file.mimeType.startsWith('video/')) return 'video';
  return null;
}

// State is reset by remount, not by an effect: callers must render
// FileThumbnail under key={file.id} (both file grids in
// /files and /files/[id] do). If a future caller breaks that contract the
// previous thumbnail would briefly flash for the wrong file — fix it by
// adding the key, not by adding a reset effect here (React's recommended
// pattern, see react.dev/learn/you-might-not-need-an-effect).
export function FileThumbnail({ file, className, iconClassName }: FileThumbnailProps) {
  const kind = thumbnailKind(file);
  const containerRef = useRef<HTMLDivElement>(null);
  const [imageSrc, setImageSrc] = useState<string | null>(null);
  const [videoSrc, setVideoSrc] = useState<string | null>(null);
  const [error, setError] = useState(false);
  const [inView, setInView] = useState(false);

  useEffect(() => {
    if (!kind) return;
    const el = containerRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          setInView(true);
          observer.disconnect();
        }
      },
      { rootMargin: '200px' },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [kind, file.id]);

  useEffect(() => {
    if (!inView || !kind) return;
    let cancelled = false;
    let objectUrl: string | null = null;

    (async () => {
      try {
        // variant=thumb steers ProxyPreview to the small derived asset
        // (image: 400px JPEG, office: PDF render). Without it the proxy
        // default for images is the full original — fine for the viewer,
        // wrong for grid tiles.
        const previewEndpoint = kind === 'image'
          ? `/files/${file.id}/preview?variant=thumb`
          : `/files/${file.id}/preview`;
        const res = await apiMethods.get<{ path: string; token: string }>(previewEndpoint);
        if (cancelled) return;
        if (!res.success) {
          setError(true);
          return;
        }
        // PreviewURL may embed its own query (?variant=thumb), so pick the
        // right separator instead of producing `...?variant=thumb?token=`.
        const sep = res.data.path.includes('?') ? '&' : '?';
        const proxyUrl = `${API_URL}${res.data.path}${sep}token=${res.data.token}`;

        if (kind === 'image') {
          // Match FileViewer: fetch as blob so the short-lived preview token
          // isn't pinned in the DOM via the <img src> attribute.
          const response = await fetch(proxyUrl);
          if (cancelled) return;
          if (!response.ok) {
            setError(true);
            return;
          }
          const blob = await response.blob();
          if (cancelled) return;
          objectUrl = URL.createObjectURL(blob);
          setImageSrc(objectUrl);
        } else if (kind === 'video') {
          // #t=0.1 nudges the browser to render the frame at 0.1s as the
          // poster instead of a blank/black box.
          setVideoSrc(`${proxyUrl}#t=0.1`);
        }
      } catch {
        if (!cancelled) setError(true);
      }
    })();

    return () => {
      cancelled = true;
      // Revoke the URL this effect run created. Cleanup fires before the
      // next effect run (on file.id / kind / inView change) and on unmount,
      // so the URL stays valid exactly as long as the <img> consuming it.
      if (objectUrl) {
        URL.revokeObjectURL(objectUrl);
      }
    };
  }, [inView, kind, file.id]);

  if (kind === 'image' && imageSrc && !error) {
    return (
      <div ref={containerRef} className={className}>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={imageSrc}
          alt=""
          className="h-full w-full object-cover"
          onError={() => setError(true)}
        />
      </div>
    );
  }

  if (kind === 'video' && videoSrc && !error) {
    return (
      <div ref={containerRef} className={className}>
        <video
          src={videoSrc}
          preload="metadata"
          muted
          playsInline
          className="pointer-events-none h-full w-full object-cover"
          onError={() => setError(true)}
        />
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className={`${className ?? ''} flex items-center justify-center`.trim()}
    >
      <FileIconComponent
        mimeType={file.mimeType}
        isDirectory={file.isDirectory}
        className={iconClassName}
      />
    </div>
  );
}
