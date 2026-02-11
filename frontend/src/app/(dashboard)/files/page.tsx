'use client';

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { File } from '@/lib/types';
import { apiMethods } from '@/lib/api';
import { downloadFile } from '@/lib/download';
import { useAuth } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
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
  Search,
  Info,
  Share2
} from 'lucide-react';
import { FileIconComponent } from '@/components/file-icon';
import { CreateFolderDialog } from '@/components/create-folder-dialog';
import { UploadZone } from '@/components/upload-zone';
import { MoveDialog } from '@/components/move-dialog';
import { FileInspector } from '@/components/file-inspector';
import { ShareDialog } from '@/components/share-dialog';
import { toast } from 'sonner';
import { format } from 'date-fns';

export default function FilesPage() {
  const { user } = useAuth();
  const [files, setFiles] = useState<File[]>([]);
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [isLoading, setIsLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [movingFile, setMovingFile] = useState<File | null>(null);
  const [sharingFile, setSharingFile] = useState<File | null>(null);
  const [inspectorOpen, setInspectorOpen] = useState(false);
  const [selectedFileId, setSelectedFileId] = useState<string | null>(null);
  const router = useRouter();

  const fetchFiles = useCallback(async () => {
    setIsLoading(true);
    try {
      const res = await apiMethods.get<File[]>('/api/files');
      if (res.success) {
        setFiles(res.data);
      }
    } catch (error) {
      console.error(error);
      toast.error('Failed to load files');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchFiles();
  }, [fetchFiles]);

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this file?')) return;
    try {
      await apiMethods.delete(`/api/files/${id}`);
      toast.success('File deleted');
      fetchFiles();
    } catch (error) {
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

  const filteredFiles = files.filter(f => 
    f.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <h1 className="text-2xl font-bold">My Files</h1>
        <div className="flex items-center gap-2">
          <div className="relative w-full md:w-64">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Search files..."
              className="pl-9"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
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
          <CreateFolderDialog onFolderCreated={fetchFiles} />
        </div>
      </div>

      <UploadZone onUploadComplete={fetchFiles} />

      {isLoading ? (
        <div className="grid grid-cols-2 gap-4 md:grid-cols-4 lg:grid-cols-5">
          {['s1', 's2', 's3', 's4', 's5'].map((key) => (
            <div key={key} className="h-32 animate-pulse rounded-lg bg-slate-200" />
          ))}
        </div>
      ) : filteredFiles.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <div className="rounded-full bg-slate-100 p-4">
            <FileIconComponent mimeType="" isDirectory={true} className="h-8 w-8 text-slate-400" />
          </div>
          <h3 className="mt-4 text-lg font-semibold">No files found</h3>
          <p className="text-sm text-muted-foreground">
            Upload files or create a folder to get started.
          </p>
        </div>
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-2 gap-4 md:grid-cols-4 lg:grid-cols-5">
          {filteredFiles.map((file) => (
            <div
              key={file.id}
              className="group relative flex flex-col justify-between rounded-lg border bg-white p-4 transition-shadow hover:shadow-md"
            >
              <Link href={`/files/${file.id}`} className="absolute inset-0 z-0" />
              <div className="flex items-start justify-between">
                <FileIconComponent 
                  mimeType={file.mimeType} 
                  isDirectory={file.isDirectory} 
                  className="h-10 w-10 text-blue-600" 
                />
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon" className="relative z-10 -mr-2 -mt-2 h-8 w-8 opacity-0 group-hover:opacity-100">
                      <MoreVertical className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={() => router.push(`/files/${file.id}`)}>
                      Open
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => {
                      setSelectedFileId(file.id);
                      setInspectorOpen(true);
                    }}>
                      <Info className="mr-2 h-4 w-4" />
                      Info
                    </DropdownMenuItem>
                    {user?.id === file.ownerID && (
                      <DropdownMenuItem onClick={() => setSharingFile(file)}>
                        <Share2 className="mr-2 h-4 w-4" />
                        Share
                      </DropdownMenuItem>
                    )}
                    {!file.isDirectory && (
                      <DropdownMenuItem onClick={() => handleDownload(file.id, file.name)}>
                        <Download className="mr-2 h-4 w-4" />
                        Download
                      </DropdownMenuItem>
                    )}
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => setMovingFile(file)}>
                      <Move className="mr-2 h-4 w-4" />
                      Move
                    </DropdownMenuItem>
                    <DropdownMenuItem className="text-red-600" onClick={() => handleDelete(file.id)}>
                      <Trash2 className="mr-2 h-4 w-4" />
                      Delete
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
              <div className="mt-4">
                <div className="flex items-center gap-2">
                  <p className="truncate font-medium text-slate-900" title={file.name}>
                    {file.name}
                  </p>
                  {file.sharedWith !== undefined && file.sharedWith > 0 && (
                    <Share2 className="h-4 w-4 text-blue-400" />
                  )}
                </div>
                <p className="text-xs text-slate-500">
                  {file.isDirectory ? 'Folder' : formatBytes(file.size)}
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
                <TableHead>Owner</TableHead>
                <TableHead>Size</TableHead>
                <TableHead>Modified</TableHead>
                <TableHead className="w-[50px]"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredFiles.map((file) => (
                <TableRow key={file.id} className="group">
                  <TableCell>
                    <FileIconComponent 
                      mimeType={file.mimeType} 
                      isDirectory={file.isDirectory} 
                      className="h-5 w-5 text-blue-600" 
                    />
                  </TableCell>
                  <TableCell className="font-medium">
                    <div className="flex items-center gap-2">
                      <Link href={`/files/${file.id}`} className="hover:underline">
                        {file.name}
                      </Link>
                      {file.sharedWith !== undefined && file.sharedWith > 0 && (
                        <Share2 className="h-4 w-4 text-blue-400" />
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Avatar className="h-6 w-6">
                        <AvatarImage src={file.owner?.avatarURL} />
                        <AvatarFallback>{file.owner?.firstName?.[0]}{file.owner?.lastName?.[0]}</AvatarFallback>
                      </Avatar>
                      <span className="truncate text-sm max-w-[100px]">
                        {file.owner ? `${file.owner.firstName} ${file.owner.lastName}` : 'Unknown'}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>{file.isDirectory ? '-' : formatBytes(file.size)}</TableCell>
                  <TableCell>{format(new Date(file.updatedAt), 'MMM d, yyyy')}</TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-8 w-8 opacity-0 group-hover:opacity-100">
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => router.push(`/files/${file.id}`)}>
                          Open
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => {
                          setSelectedFileId(file.id);
                          setInspectorOpen(true);
                        }}>
                          <Info className="mr-2 h-4 w-4" />
                          Info
                        </DropdownMenuItem>
                        {user?.id === file.ownerID && (
                          <DropdownMenuItem onClick={() => setSharingFile(file)}>
                            <Share2 className="mr-2 h-4 w-4" />
                            Share
                          </DropdownMenuItem>
                        )}
                        {!file.isDirectory && (
                          <DropdownMenuItem onClick={() => handleDownload(file.id, file.name)}>
                            <Download className="mr-2 h-4 w-4" />
                            Download
                          </DropdownMenuItem>
                        )}
                        <DropdownMenuSeparator />
                        <DropdownMenuItem onClick={() => setMovingFile(file)}>
                          <Move className="mr-2 h-4 w-4" />
                          Move
                        </DropdownMenuItem>
                        <DropdownMenuItem className="text-red-600" onClick={() => handleDelete(file.id)}>
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
          onMoved={fetchFiles}
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
        fileId={selectedFileId} 
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
