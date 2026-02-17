'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { File } from '@/lib/types';
import { apiMethods } from '@/lib/api';

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { FileIconComponent } from '@/components/file-icon';
import { Loading } from '@/components/loading';
import { toast } from 'sonner';
import { format } from 'date-fns';

function formatBytes(bytes: number) {
  if (!+bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

export default function SharedWithMePage() {
  const [files, setFiles] = useState<File[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const fetchShared = async () => {
      try {
        const res = await apiMethods.get<File[]>('/shared');
        if (res.success) {
          setFiles(Array.isArray(res.data) ? res.data : []);
        }
      } catch {
        toast.error('Failed to load shared files');
      } finally {
        setIsLoading(false);
      }
    };
    fetchShared();
  }, []);

  if (isLoading) return <Loading />;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Shared With Me</h1>

      {files.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <h3 className="text-lg font-semibold">No shared files</h3>
          <p className="text-sm text-muted-foreground">
            Files shared with you will appear here.
          </p>
        </div>
      ) : (
        <div className="rounded-md border bg-white">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[40px]"></TableHead>
                <TableHead>Name</TableHead>
                <TableHead>Owner</TableHead>
                <TableHead>Size</TableHead>
                <TableHead>Shared</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {files.map((file) => (
                <TableRow key={file.id}>
                  <TableCell>
                    <FileIconComponent mimeType={file.mimeType} isDirectory={file.isDirectory} className="h-5 w-5 text-blue-600" />
                  </TableCell>
                  <TableCell className="font-medium">
                    <Link href={`/files/${file.id}`} className="hover:underline">
                      {file.name}
                    </Link>
                  </TableCell>
                  <TableCell>
                    {file.owner ? (
                      <div className="flex items-center gap-2">
                        <Avatar className="h-6 w-6">
                          <AvatarImage src={file.owner.avatarURL} />
                          <AvatarFallback>{file.owner.firstName?.[0]}{file.owner.lastName?.[0]}</AvatarFallback>
                        </Avatar>
                        <span className="truncate text-sm max-w-[120px]">
                          {file.owner.firstName} {file.owner.lastName}
                        </span>
                      </div>
                    ) : (
                      <span className="text-sm text-muted-foreground">Unknown</span>
                    )}
                  </TableCell>
                  <TableCell>{file.isDirectory ? '-' : formatBytes(file.size)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {format(new Date(file.createdAt), 'MMM d, yyyy')}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
