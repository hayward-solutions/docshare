import { create } from 'zustand';
import { apiMethods, filesAPI, putToPresignedURL, UploadAbortError } from './api';

export type UploadStatus =
  | 'queued'
  | 'presigning'
  | 'uploading'
  | 'finalizing'
  | 'completed'
  | 'error'
  | 'cancelled';

export interface UploadItem {
  id: string;
  file: File;
  name: string;
  size: number;
  mimeType: string;
  parentID: string | null;
  parentLabel?: string;
  progress: number;
  status: UploadStatus;
  error?: string;
  remoteID?: string;
  abortController?: AbortController;
  addedAt: number;
  completedAt?: number;
}

export interface UploadContext {
  parentID: string | null;
  canUpload: boolean;
  label?: string;
}

interface UploadState {
  items: Record<string, UploadItem>;
  order: string[];

  isModalOpen: boolean;
  modalParentID: string | null;
  modalParentLabel?: string;

  currentContext: UploadContext | null;

  parentCompletionTicks: Record<string, number>;
  totalCompletions: number;
  totalErrors: number;

  setCurrentContext: (ctx: UploadContext | null) => void;
  openModal: () => void;
  closeModal: () => void;

  addFiles: (files: File[]) => void;
  cancel: (id: string) => void;
  retry: (id: string) => void;
  remove: (id: string) => void;
  clearFinished: () => void;
}

const CONCURRENCY = 3;

export const ROOT_KEY = '__root__';
export function parentKey(parentID: string | null | undefined): string {
  return parentID ?? ROOT_KEY;
}

function isActive(status: UploadStatus): boolean {
  return status === 'presigning' || status === 'uploading' || status === 'finalizing';
}

function isPendingOrActive(status: UploadStatus): boolean {
  return status === 'queued' || isActive(status);
}

function newID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `u_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

export const useUploadStore = create<UploadState>((set, get) => {
  function patchItem(id: string, patch: Partial<UploadItem>) {
    set((state) => {
      const existing = state.items[id];
      if (!existing) return state;
      return { items: { ...state.items, [id]: { ...existing, ...patch } } };
    });
  }

  function tickParent(parentID: string | null) {
    set((state) => {
      const key = parentKey(parentID);
      const next = (state.parentCompletionTicks[key] ?? 0) + 1;
      return {
        parentCompletionTicks: { ...state.parentCompletionTicks, [key]: next },
        totalCompletions: state.totalCompletions + 1,
      };
    });
  }

  async function runOne(id: string) {
    const initial = get().items[id];
    if (!initial || initial.status !== 'queued') return;

    const controller = new AbortController();
    patchItem(id, { status: 'presigning', abortController: controller, error: undefined });

    try {
      const presigned = await filesAPI.presignUpload({
        name: initial.name,
        size: initial.size,
        mimeType: initial.mimeType,
        parentID: initial.parentID,
      });
      if (controller.signal.aborted) throw new UploadAbortError();
      if (!presigned.success) {
        throw new Error(presigned.error || 'failed to obtain upload URL');
      }

      patchItem(id, { status: 'uploading', progress: 0 });

      await putToPresignedURL(presigned.data.uploadURL, initial.file, {
        signal: controller.signal,
        onProgress: (loaded, total) => {
          const pct = total > 0 ? Math.round((loaded / total) * 100) : 0;
          patchItem(id, { progress: pct });
        },
      });

      if (controller.signal.aborted) throw new UploadAbortError();

      patchItem(id, { status: 'finalizing', progress: 100 });

      const finalized = await filesAPI.finalizeUpload({
        key: presigned.data.key,
        name: initial.name,
        mimeType: initial.mimeType,
        parentID: initial.parentID,
      });
      if (!finalized.success) {
        throw new Error(finalized.error || 'failed to finalize upload');
      }

      if (controller.signal.aborted) {
        void apiMethods.delete(`/files/${finalized.data.id}`).catch(() => {});
        throw new UploadAbortError();
      }

      patchItem(id, {
        status: 'completed',
        progress: 100,
        remoteID: finalized.data.id,
        completedAt: Date.now(),
        abortController: undefined,
      });
      tickParent(initial.parentID);
    } catch (err) {
      if (err instanceof UploadAbortError || controller.signal.aborted) {
        patchItem(id, { status: 'cancelled', abortController: undefined });
      } else {
        const message = err instanceof Error ? err.message : 'upload failed';
        patchItem(id, { status: 'error', error: message, abortController: undefined });
        set((state) => ({ totalErrors: state.totalErrors + 1 }));
      }
    } finally {
      runQueue();
    }
  }

  function runQueue() {
    const { items, order } = get();
    const active = order
      .map((id) => items[id])
      .filter((it): it is UploadItem => !!it && isActive(it.status));
    let slots = CONCURRENCY - active.length;
    if (slots <= 0) return;

    for (const id of order) {
      if (slots <= 0) break;
      const item = items[id];
      if (item && item.status === 'queued') {
        slots -= 1;
        void runOne(id);
      }
    }
  }

  return {
    items: {},
    order: [],

    isModalOpen: false,
    modalParentID: null,
    modalParentLabel: undefined,

    currentContext: null,

    parentCompletionTicks: {},
    totalCompletions: 0,
    totalErrors: 0,

    setCurrentContext: (ctx) => set({ currentContext: ctx }),

    openModal: () => {
      const ctx = get().currentContext;
      if (!ctx?.canUpload) return;
      set({
        isModalOpen: true,
        modalParentID: ctx.parentID,
        modalParentLabel: ctx.label,
      });
    },

    closeModal: () => set({ isModalOpen: false }),

    addFiles: (files) => {
      if (files.length === 0) return;
      const state = get();
      const parentID = state.modalParentID;
      const parentLabel = state.modalParentLabel;
      const now = Date.now();

      const newItems: Record<string, UploadItem> = {};
      const newIds: string[] = [];
      for (const file of files) {
        const id = newID();
        newItems[id] = {
          id,
          file,
          name: file.name,
          size: file.size,
          mimeType: file.type || 'application/octet-stream',
          parentID,
          parentLabel,
          progress: 0,
          status: 'queued',
          addedAt: now,
        };
        newIds.push(id);
      }

      set({
        items: { ...state.items, ...newItems },
        order: [...state.order, ...newIds],
        isModalOpen: false,
      });

      runQueue();
    },

    cancel: (id) => {
      const item = get().items[id];
      if (!item) return;
      if (item.status === 'queued') {
        patchItem(id, { status: 'cancelled' });
        return;
      }
      if (isActive(item.status) && item.abortController) {
        item.abortController.abort();
      }
    },

    retry: (id) => {
      const item = get().items[id];
      if (!item) return;
      if (item.status !== 'error' && item.status !== 'cancelled') return;
      patchItem(id, { status: 'queued', progress: 0, error: undefined });
      runQueue();
    },

    remove: (id) => {
      const item = get().items[id];
      if (!item) return;
      if (isActive(item.status)) {
        item.abortController?.abort();
      }
      set((state) => {
        const { [id]: _omit, ...rest } = state.items;
        void _omit;
        return {
          items: rest,
          order: state.order.filter((existing) => existing !== id),
        };
      });
    },

    clearFinished: () => {
      set((state) => {
        const keepItems: Record<string, UploadItem> = {};
        const keepOrder: string[] = [];
        for (const id of state.order) {
          const item = state.items[id];
          if (!item) continue;
          if (isPendingOrActive(item.status) || item.status === 'error') {
            keepItems[id] = item;
            keepOrder.push(id);
          }
        }
        return { items: keepItems, order: keepOrder };
      });
    },
  };
});

export function selectVisibleItems(state: UploadState): UploadItem[] {
  return state.order.map((id) => state.items[id]).filter((it): it is UploadItem => !!it);
}

export function selectActiveCount(state: UploadState): number {
  let n = 0;
  for (const id of state.order) {
    const item = state.items[id];
    if (item && isPendingOrActive(item.status)) n += 1;
  }
  return n;
}

export function selectHasItems(state: UploadState): boolean {
  return state.order.length > 0;
}
