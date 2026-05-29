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
  if (file.mimeType.startsWith('image/')) return 'image';
  if (file.mimeType.startsWith('video/')) return 'video';
  return null;
}

export function FileThumbnail({ file, className, iconClassName }: FileThumbnailProps) {
  const kind = thumbnailKind(file);
  const containerRef = useRef<HTMLDivElement>(null);
  const blobUrlRef = useRef<string | null>(null);
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
  }, [kind]);

  useEffect(() => {
    if (!inView || !kind) return;
    let cancelled = false;

    (async () => {
      try {
        const res = await apiMethods.get<{ path: string; token: string }>(
          `/files/${file.id}/preview`,
        );
        if (cancelled) return;
        if (!res.success) {
          setError(true);
          return;
        }
        const proxyUrl = `${API_URL}${res.data.path}?token=${res.data.token}`;

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
          const objectUrl = URL.createObjectURL(blob);
          blobUrlRef.current = objectUrl;
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
    };
  }, [inView, kind, file.id]);

  useEffect(() => {
    return () => {
      if (blobUrlRef.current) {
        URL.revokeObjectURL(blobUrlRef.current);
        blobUrlRef.current = null;
      }
    };
  }, []);

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
