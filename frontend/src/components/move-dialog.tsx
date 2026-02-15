'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { apiMethods } from '@/lib/api';
import { File } from '@/lib/types';
import { toast } from 'sonner';
import { ChevronRight, Folder, Loader2 } from 'lucide-react';

const ROOT_VALUE = '__root__';

interface DirectoryNode {
  id: string;
  name: string;
  parentID?: string;
  children: DirectoryNode[];
}

interface FolderOption {
  id: string;
  name: string;
  depth: number;
}

interface MoveDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  fileId?: string;
  fileIds?: string[];
  fileName: string;
  currentParentID?: string;
  onMoved: () => void;
}

async function fetchDirectoryTree(dir: File): Promise<DirectoryNode> {
  const childrenRes = await apiMethods.get<File[]>(`/api/files/${dir.id}/children`);
  const childDirectories = childrenRes.success
    ? childrenRes.data.filter((child: File) => child.isDirectory)
    : [];

  const children = await Promise.all(childDirectories.map((child: File) => fetchDirectoryTree(child)));

  return {
    id: dir.id,
    name: dir.name,
    parentID: dir.parentID,
    children,
  };
}

function flattenTree(nodes: DirectoryNode[], depth = 0): FolderOption[] {
  return nodes.flatMap((node) => [
    { id: node.id, name: node.name, depth },
    ...flattenTree(node.children, depth + 1),
  ]);
}

function findNode(nodes: DirectoryNode[], id: string): DirectoryNode | null {
  for (const node of nodes) {
    if (node.id === id) {
      return node;
    }

    const childMatch = findNode(node.children, id);
    if (childMatch) {
      return childMatch;
    }
  }

  return null;
}

function collectDescendantIDs(node: DirectoryNode): Set<string> {
  const ids = new Set<string>();

  const walk = (current: DirectoryNode) => {
    ids.add(current.id);
    current.children.forEach(walk);
  };

  walk(node);
  return ids;
}

export function MoveDialog({
  open,
  onOpenChange,
  fileId,
  fileIds,
  fileName,
  currentParentID,
  onMoved,
}: MoveDialogProps) {
  const resolvedIds = useMemo(() => fileIds ?? (fileId ? [fileId] : []), [fileIds, fileId]);
  const [directories, setDirectories] = useState<DirectoryNode[]>([]);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [selectedFolderID, setSelectedFolderID] = useState('');
  const [isLoadingTree, setIsLoadingTree] = useState(false);
  const [isMoving, setIsMoving] = useState(false);

  const loadDirectories = useCallback(async () => {
    setIsLoadingTree(true);
    try {
      const rootRes = await apiMethods.get<File[]>('/api/files');
      if (!rootRes.success) {
        throw new Error('Failed to load folders');
      }

      const rootDirectories = rootRes.data.filter((entry: File) => entry.isDirectory);
      const tree = await Promise.all(rootDirectories.map((dir: File) => fetchDirectoryTree(dir)));

      setDirectories(tree);
      setExpanded(Object.fromEntries(rootDirectories.map((dir: File) => [dir.id, true])));
    } catch (error) {
      console.error(error);
      toast.error('Failed to load folders');
      setDirectories([]);
    } finally {
      setIsLoadingTree(false);
    }
  }, []);

  useEffect(() => {
    if (!open) {
      return;
    }

    setSelectedFolderID(currentParentID ?? '');
    loadDirectories();
  }, [open, currentParentID, loadDirectories]);

  const blockedFolderIDs = useMemo(() => {
    const blocked = new Set<string>();
    for (const id of resolvedIds) {
      const node = findNode(directories, id);
      if (node) {
        for (const descId of collectDescendantIDs(node)) {
          blocked.add(descId);
        }
      } else {
        blocked.add(id);
      }
    }
    return blocked;
  }, [directories, resolvedIds]);

  const folderOptions = useMemo(
    () => flattenTree(directories).filter((option) => !blockedFolderIDs.has(option.id)),
    [directories, blockedFolderIDs]
  );

  const selectedParentName =
    selectedFolderID === ''
      ? 'My Files (Root)'
      : folderOptions.find((option) => option.id === selectedFolderID)?.name ?? null;

  const toggleExpanded = (id: string) => {
    setExpanded((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  const handleMove = async () => {
    setIsMoving(true);
    try {
      await Promise.all(
        resolvedIds.map((id) =>
          apiMethods.put(`/api/files/${id}`, { parentID: selectedFolderID }),
        ),
      );
      toast.success(resolvedIds.length > 1 ? `${resolvedIds.length} items moved` : 'Moved successfully');
      onOpenChange(false);
      onMoved();
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : 'Failed to move item');
    } finally {
      setIsMoving(false);
    }
  };

  const isUnchangedDestination =
    resolvedIds.length === 1 && (currentParentID ?? '') === selectedFolderID;

  const renderTree = (nodes: DirectoryNode[], depth = 0) =>
    nodes
      .filter((node) => !blockedFolderIDs.has(node.id))
      .map((node) => {
        const hasChildren = node.children.length > 0;
        const isExpanded = expanded[node.id] ?? false;
        const isSelected = selectedFolderID === node.id;

        return (
          <div key={node.id} className="space-y-1">
            <div
              className={`flex items-center gap-1 rounded-md border px-2 py-1 ${
                isSelected ? 'border-blue-500 bg-blue-50' : 'border-transparent hover:bg-slate-50'
              }`}
              style={{ paddingLeft: `${depth * 16 + 8}px` }}
            >
              <button
                type="button"
                className="inline-flex h-6 w-6 items-center justify-center rounded-sm hover:bg-slate-200 disabled:opacity-40"
                disabled={!hasChildren}
                onClick={() => hasChildren && toggleExpanded(node.id)}
              >
                <ChevronRight
                  className={`h-4 w-4 transition-transform ${isExpanded ? 'rotate-90' : ''}`}
                />
              </button>

              <button
                type="button"
                className="flex min-w-0 flex-1 items-center gap-2 rounded-sm px-1 py-1 text-left"
                onClick={() => setSelectedFolderID(node.id)}
              >
                <Folder className="h-4 w-4 shrink-0 text-blue-600" />
                <span className="truncate text-sm">{node.name}</span>
              </button>
            </div>

            {hasChildren && isExpanded ? renderTree(node.children, depth + 1) : null}
          </div>
        );
      });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>Move &quot;{fileName}&quot;</DialogTitle>
          <DialogDescription>Select a destination folder.</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <Select
            value={selectedFolderID === '' ? ROOT_VALUE : selectedFolderID}
            onValueChange={(value) => setSelectedFolderID(value === ROOT_VALUE ? '' : value)}
          >
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Select destination folder" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ROOT_VALUE}>My Files (Root)</SelectItem>
              {folderOptions.map((option) => (
                <SelectItem key={option.id} value={option.id}>
                  {`${'  '.repeat(option.depth)}${option.name}`}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <div className="max-h-[280px] overflow-y-auto rounded-md border p-2">
            <button
              type="button"
              className={`mb-2 w-full rounded-md border px-2 py-2 text-left text-sm ${
                selectedFolderID === ''
                  ? 'border-blue-500 bg-blue-50 font-medium'
                  : 'border-transparent hover:bg-slate-50'
              }`}
              onClick={() => setSelectedFolderID('')}
            >
              My Files (Root)
            </button>

            {isLoadingTree ? (
              <div className="flex items-center justify-center py-8 text-sm text-muted-foreground">
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Loading folders...
              </div>
            ) : (
              <div className="space-y-1">{renderTree(directories)}</div>
            )}
          </div>

          <p className="text-sm text-muted-foreground">
            Destination: <span className="font-medium text-slate-900">{selectedParentName ?? 'None'}</span>
          </p>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleMove} disabled={isMoving || isLoadingTree || isUnchangedDestination}>
            {isMoving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Move
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
