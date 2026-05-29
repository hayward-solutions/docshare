'use client';

import { useEffect, useRef, useState } from 'react';
import { File } from '@/lib/types';
import { apiMethods, previewAPI } from '@/lib/api';
import { FileIconComponent } from './file-icon';

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? '';

interface FileThumbnailProps {
  file: File;
  className?: string;
  iconClassName?: string;
}

type ThumbKind = 'image' | 'video' | null;

// True when the file is an image whose thumbnail might still be generating
// server-side: the upload-completion refetch can arrive before the
// preview job finishes (job is pending or processing). Polling
// preview-status from here flips the tile from icon → thumbnail without
// the user reloading or changing folders.
function isAwaitingImageThumbnail(file: File): boolean {
  return !file.isDirectory
    && !file.thumbnailPath
    && file.mimeType.startsWith('image/');
}

function baseKind(file: File): ThumbKind {
  if (file.isDirectory) return null;
  // Both image and video tiles only render when the server has produced
  // a small derived asset. Without this gate images without a thumbnail
  // would stream the full original (ProxyPreview falls back to
  // StoragePath) and videos would receive the full file from the
  // Range-less SendStream. The image polling effect below promotes
  // 'await' → 'image' once the job completes.
  if (!file.thumbnailPath) return null;
  if (file.mimeType.startsWith('image/')) return 'image';
  if (file.mimeType.startsWith('video/')) return 'video';
  return null;
}

const STATUS_POLL_INTERVAL_MS = 3000;
const STATUS_POLL_MAX_ATTEMPTS = 30; // ≈90s total

// State is reset by remount, not by an effect: callers must render
// FileThumbnail under key={file.id} (both file grids in
// /files and /files/[id] do). If a future caller breaks that contract the
// previous thumbnail would briefly flash for the wrong file — fix it by
// adding the key, not by adding a reset effect here (React's recommended
// pattern, see react.dev/learn/you-might-not-need-an-effect).
export function FileThumbnail({ file, className, iconClassName }: FileThumbnailProps) {
  const propKind = baseKind(file);
  const awaiting = isAwaitingImageThumbnail(file);
  // polledReady stores the result of /preview-status polling locally so a
  // completed job promotes the tile to 'image' without waiting for the
  // parent to refetch.
  const [polledReady, setPolledReady] = useState(false);
  const kind: ThumbKind = propKind ?? (polledReady ? 'image' : null);
  const shouldObserve = kind !== null || awaiting;

  const containerRef = useRef<HTMLDivElement>(null);
  const [imageSrc, setImageSrc] = useState<string | null>(null);
  const [videoSrc, setVideoSrc] = useState<string | null>(null);
  const [error, setError] = useState(false);
  const [inView, setInView] = useState(false);

  // Lazy load: observe for both kind=image/video (fetch the thumbnail)
  // and awaiting (start polling). When the poll resolves and kind flips
  // from null to 'image', this effect re-runs and re-observes — the
  // element is already in view so inView stays true and the fetch
  // effect picks it up.
  useEffect(() => {
    if (!shouldObserve) return;
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
  }, [shouldObserve, file.id]);

  // Poll preview-status for images whose thumbnail is still being
  // generated. Stops on completion (sets polledReady → kind becomes
  // 'image'), terminal failure, or attempt cap.
  useEffect(() => {
    if (!inView || !awaiting || polledReady) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let attempts = 0;

    const tick = async () => {
      attempts += 1;
      try {
        const res = await previewAPI.getStatus(file.id);
        if (cancelled) return;
        if (res.success) {
          const f = (res.data as { file?: { thumbnailPath?: string } }).file;
          if (f?.thumbnailPath) {
            setPolledReady(true);
            return;
          }
          const job = res.data.job;
          // Terminal states: stop polling, leave the icon. The grid will
          // pick up the thumbnail next time the file is fetched.
          if (!job || job.status === 'failed' || job.status === 'completed') {
            return;
          }
        }
      } catch {
        // Network blip: treat as a pending tick and retry below.
      }
      if (attempts < STATUS_POLL_MAX_ATTEMPTS && !cancelled) {
        timer = setTimeout(tick, STATUS_POLL_INTERVAL_MS);
      }
    };
    tick();

    return () => {
      cancelled = true;
      if (timer !== null) clearTimeout(timer);
    };
  }, [inView, awaiting, polledReady, file.id]);

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
