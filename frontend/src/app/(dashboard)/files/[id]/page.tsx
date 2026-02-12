'use client';

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { useRouter, useParams } from 'next/navigation';
import { File, BreadcrumbItem } from '@/lib/types';
import { apiMethods } from '@/lib/api';
import { downloadFile } from '@/lib/download';
import { useAuth } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { 
  LayoutGrid, 
  List, 
  MoreVertical, 
  Download, 
  Trash2, 
  Move,
  ChevronRight,
  ArrowLeft,
  Info,
  Share2
} from 'lucide-react';
import { FileIconComponent } from '@/components/file-icon';
import { CreateFolderDialog } from '@/components/create-folder-dialog';
import { UploadZone } from '@/components/upload-zone';
import { ShareDialog } from '@/components/share-dialog';
import { MoveDialog } from '@/components/move-dialog';
import { FileViewer } from '@/components/file-viewer';
import { FileInspector } from '@/components/file-inspector';
import { toast } from 'sonner';
import { format } from 'date-fns';
import { Loading } from '@/components/loading';

export default function FileDetailPage() {
  const { user } = useAuth();
  const params = useParams();
  const id = params.id as string;
  const router = useRouter();

  const [file, setFile] = useState<File | null>(null);
  const [children, setChildren] = useState<File[]>([]);
  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>([]);
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [isLoading, setIsLoading] = useState(true);
  const [movingFile, setMovingFile] = useState<File | null>(null);
  const [sharingFile, setSharingFile] = useState<File | null>(null);
  const [inspectorOpen, setInspectorOpen] = useState(false);

  const fetchData = useCallback(async () => {
    setIsLoading(true);
    try {
      const fileRes = await apiMethods.get<File>(`/api/files/${id}`);
      if (!fileRes.success) throw new Error('Failed to load file');
      setFile(fileRes.data);

      const pathRes = await apiMethods.get<BreadcrumbItem[]>(`/api/files/${id}/path`);
      if (pathRes.success) {
        setBreadcrumbs(pathRes.data);
      }

      if (fileRes.data.isDirectory) {
        const childrenRes = await apiMethods.get<File[]>(`/api/files/${id}/children`);
        if (childrenRes.success) {
          setChildren(childrenRes.data);
        }
      }
    } catch {
      toast.error('Failed to load file data');
      router.push('/files');
    } finally {
      setIsLoading(false);
    }
  }, [id, router]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleDelete = async (fileId: string) => {
    if (!confirm('Are you sure you want to delete this file?')) return;
    try {
      await apiMethods.delete(`/api/files/${fileId}`);
      toast.success('File deleted');
      if (fileId === id) {
        router.push(file?.parentID ? `/files/${file.parentID}` : '/files');
      } else {
        fetchData();
      }
    } catch {
      toast.error('Failed to delete file');
    }
  };

const handleDownload = async (fileId: string, fileName: string) => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : undefined;
    await downloadFile({
      url: `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/files/${fileId}/download`,
      filename: fileName,
      token: token || undefined,
    });
  };

  if (isLoading) return <Loading />;
  if (!file) return null;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm text-slate-500">
        <Link href="/files" className="hover:text-slate-900">My Files</Link>
        {breadcrumbs.map((crumb) => (
          <div key={crumb.id} className="flex items-center gap-2">
            <ChevronRight className="h-4 w-4" />
            <Link href={`/files/${crumb.id}`} className="hover:text-slate-900">
              {crumb.name}
            </Link>
          </div>
        ))}
        <div className="flex items-center gap-2">
          <ChevronRight className="h-4 w-4" />
          <span className="font-medium text-slate-900">{file.name}</span>
        </div>
      </div>

      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" onClick={() => router.back()}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <FileIconComponent mimeType={file.mimeType} isDirectory={file.isDirectory} className="h-8 w-8 text-blue-600" />
            {file.name}
          </h1>
        </div>
        
        <div className="flex items-center gap-2">
          {!file.isDirectory && (
            <Button variant="outline" onClick={() => handleDownload(file.id, file.name)}>
              <Download className="mr-2 h-4 w-4" />
              Download
            </Button>
          )}

          {user?.id === file.ownerID && (
            <ShareDialog fileId={file.id} fileName={file.name} />
          )}
          
          <Button variant="ghost" size="icon" onClick={() => setInspectorOpen(true)}>
            <Info className="h-4 w-4" />
          </Button>

          {file.isDirectory && (
            <>
              <div className="flex items-center border rounded-md bg-white">
                <Button
                  variant="ghost"
                  size="icon"
                  className={`rounded-none rounded-l-md ${viewMode === 'grid' ? 'bg-slate-100' : ''}`}
                  onClick={() => setViewMode('grid')}
                >
                  <LayoutGrid className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className={`rounded-none rounded-r-md ${viewMode === 'list' ? 'bg-slate-100' : ''}`}
                  onClick={() => setViewMode('list')}
                >
                  <List className="h-4 w-4" />
                </Button>
              </div>
              <CreateFolderDialog parentID={file.id} onFolderCreated={fetchData} />
            </>
          )}
          
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon">
                <MoreVertical className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => setMovingFile(file)}>
                <Move className="mr-2 h-4 w-4" />
                Move
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-red-600" onClick={() => handleDelete(file.id)}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {file.isDirectory ? (
        <>
          <UploadZone parentID={file.id} onUploadComplete={fetchData} />
          
          {children.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <div className="rounded-full bg-slate-100 p-4">
                <FileIconComponent mimeType="" isDirectory={true} className="h-8 w-8 text-slate-400" />
              </div>
              <h3 className="mt-4 text-lg font-semibold">Empty folder</h3>
              <p className="text-sm text-muted-foreground">
                Upload files or create a subfolder.
              </p>
            </div>
          ) : viewMode === 'grid' ? (
            <div className="grid grid-cols-2 gap-4 md:grid-cols-4 lg:grid-cols-5">
              {children.map((child) => (
                <div
                  key={child.id}
                  className="group relative flex flex-col justify-between rounded-lg border bg-white p-4 transition-shadow hover:shadow-md"
                >
                  <Link href={`/files/${child.id}`} className="absolute inset-0 z-0" />
                  <div className="flex items-start justify-between">
                    <FileIconComponent 
                      mimeType={child.mimeType} 
                      isDirectory={child.isDirectory} 
                      className="h-10 w-10 text-blue-600" 
                    />
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="relative z-10 -mr-2 -mt-2 h-8 w-8 opacity-0 group-hover:opacity-100">
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => router.push(`/files/${child.id}`)}>
                          Open
                        </DropdownMenuItem>
                        {user?.id === child.ownerID && (
                          <DropdownMenuItem onClick={() => setSharingFile(child)}>
                            <Share2 className="mr-2 h-4 w-4" />
                            Share
                          </DropdownMenuItem>
                        )}
                        {!child.isDirectory && (
                          <DropdownMenuItem onClick={() => handleDownload(child.id, child.name)}>
                            <Download className="mr-2 h-4 w-4" />
                            Download
                          </DropdownMenuItem>
                        )}
                        <DropdownMenuSeparator />
                        <DropdownMenuItem onClick={() => setMovingFile(child)}>
                          <Move className="mr-2 h-4 w-4" />
                          Move
                        </DropdownMenuItem>
                        <DropdownMenuItem className="text-red-600" onClick={() => handleDelete(child.id)}>
                          <Trash2 className="mr-2 h-4 w-4" />
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                  <div className="mt-4">
                    <p className="truncate font-medium text-slate-900" title={child.name}>
                      {child.name}
                    </p>
                    <p className="text-xs text-slate-500">
                      {child.isDirectory ? 'Folder' : formatBytes(child.size)}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="rounded-md border bg-white">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[50px]"></TableHead>
                    <TableHead>Name</TableHead>
                    <TableHead>Size</TableHead>
                    <TableHead>Modified</TableHead>
                    <TableHead className="w-[50px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {children.map((child) => (
                    <TableRow key={child.id} className="group">
                      <TableCell>
                        <FileIconComponent 
                          mimeType={child.mimeType} 
                          isDirectory={child.isDirectory} 
                          className="h-5 w-5 text-blue-600" 
                        />
                      </TableCell>
                      <TableCell className="font-medium">
                        <Link href={`/files/${child.id}`} className="hover:underline">
                          {child.name}
                        </Link>
                      </TableCell>
                      <TableCell>{child.isDirectory ? '-' : formatBytes(child.size)}</TableCell>
                      <TableCell>{format(new Date(child.updatedAt), 'MMM d, yyyy')}</TableCell>
                      <TableCell>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button variant="ghost" size="icon" className="h-8 w-8 opacity-0 group-hover:opacity-100">
                              <MoreVertical className="h-4 w-4" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem onClick={() => router.push(`/files/${child.id}`)}>
                              Open
                            </DropdownMenuItem>
                            {user?.id === child.ownerID && (
                              <DropdownMenuItem onClick={() => setSharingFile(child)}>
                                <Share2 className="mr-2 h-4 w-4" />
                                Share
                              </DropdownMenuItem>
                            )}
                            {!child.isDirectory && (
                              <DropdownMenuItem onClick={() => handleDownload(child.id, child.name)}>
                                <Download className="mr-2 h-4 w-4" />
                                Download
                              </DropdownMenuItem>
                            )}
                            <DropdownMenuSeparator />
                            <DropdownMenuItem onClick={() => setMovingFile(child)}>
                              <Move className="mr-2 h-4 w-4" />
                              Move
                            </DropdownMenuItem>
                            <DropdownMenuItem className="text-red-600" onClick={() => handleDelete(child.id)}>
                              <Trash2 className="mr-2 h-4 w-4" />
                              Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </>
      ) : (
        <FileViewer file={file} />
      )}

      {movingFile && (
        <MoveDialog
          open={!!movingFile}
          onOpenChange={(open) => {
            if (!open) {
              setMovingFile(null);
            }
          }}
          fileId={movingFile.id}
          fileName={movingFile.name}
          isDirectory={movingFile.isDirectory}
          currentParentID={movingFile.parentID}
          onMoved={fetchData}
        />
      )}

      {sharingFile && (
        <ShareDialog
          open={!!sharingFile}
          onOpenChange={(open) => !open && setSharingFile(null)}
          fileId={sharingFile.id}
          fileName={sharingFile.name}
        />
      )}

      <FileInspector 
        open={inspectorOpen} 
        onOpenChange={setInspectorOpen} 
        fileId={file.id} 
      />
    </div>
  );
}

function formatBytes(bytes: number, decimals = 2) {
  if (!+bytes) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}
