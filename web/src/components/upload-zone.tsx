'use client';

import { useCallback, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { Upload, X, File as FileIcon, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { filesAPI, putToPresignedURL } from '@/lib/api';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import { useActivityToast } from '@/hooks/use-activity-toast';

interface UploadZoneProps {
  parentID?: string;
  onUploadComplete: () => void;
}

interface UploadingFile {
  file: File;
  progress: number;
  status: 'pending' | 'uploading' | 'completed' | 'error';
}

export function UploadZone({ parentID, onUploadComplete }: UploadZoneProps) {
  const [uploadingFiles, setUploadingFiles] = useState<UploadingFile[]>([]);
  const { successWithRefresh } = useActivityToast();

  const uploadFileToServer = useCallback(async (uploadFile: UploadingFile) => {
    setUploadingFiles(prev => prev.map(f =>
      f.file === uploadFile.file ? { ...f, status: 'uploading' } : f
    ));

    const file = uploadFile.file;
    const mimeType = file.type || 'application/octet-stream';

    try {
      const presigned = await filesAPI.presignUpload({
        name: file.name,
        size: file.size,
        mimeType,
        parentID: parentID ?? null,
      });
      if (!presigned.success) {
        throw new Error(presigned.error || 'failed to obtain upload URL');
      }

      await putToPresignedURL(presigned.data.uploadURL, file, (loaded, total) => {
        const pct = total > 0 ? Math.round((loaded / total) * 100) : 0;
        setUploadingFiles(prev => prev.map(f =>
          f.file === file ? { ...f, progress: pct } : f
        ));
      });

      const finalized = await filesAPI.finalizeUpload({
        key: presigned.data.key,
        name: file.name,
        mimeType,
        parentID: parentID ?? null,
      });
      if (!finalized.success) {
        throw new Error(finalized.error || 'failed to finalize upload');
      }

      setUploadingFiles(prev => prev.map(f =>
        f.file === file ? { ...f, progress: 100, status: 'completed' } : f
      ));
      successWithRefresh(`Uploaded ${file.name}`);
    } catch (error) {
      console.error(error);
      setUploadingFiles(prev => prev.map(f =>
        f.file === file ? { ...f, status: 'error' } : f
      ));
      toast.error(`Failed to upload ${file.name}`);
    }
  }, [parentID, successWithRefresh]);

  const onDrop = useCallback(async (acceptedFiles: File[]) => {
    const newFiles = acceptedFiles.map(file => ({
      file,
      progress: 0,
      status: 'pending' as const
    }));

    setUploadingFiles(prev => [...prev, ...newFiles]);

    for (const uploadFile of newFiles) {
      await uploadFileToServer(uploadFile);
    }
    
    onUploadComplete();
  }, [uploadFileToServer, onUploadComplete]);

  const removeFile = (fileToRemove: File) => {
    setUploadingFiles(prev => prev.filter(f => f.file !== fileToRemove));
  };

  const { getRootProps, getInputProps, isDragActive } = useDropzone({ onDrop });

  return (
    <div className="space-y-4">
      <div
        {...getRootProps()}
        className={cn(
          "border-2 border-dashed rounded-lg p-8 text-center cursor-pointer transition-colors",
          isDragActive ? "border-primary bg-primary/5" : "border-border hover:border-primary/50"
        )}
      >
        <input {...getInputProps()} />
        <div className="flex flex-col items-center gap-2 text-muted-foreground">
          <Upload className="h-8 w-8" />
          <p className="text-sm font-medium">
            {isDragActive ? "Drop files here" : "Drag & drop files here, or click to select"}
          </p>
        </div>
      </div>

      {uploadingFiles.length > 0 && (
        <div className="space-y-2">
          {uploadingFiles.map((item) => (
            <div key={item.file.name + item.file.size} className="flex items-center gap-3 rounded-lg border p-3 bg-card">
              <FileIcon className="h-8 w-8 text-blue-500 dark:text-blue-400" />
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between mb-1">
                  <p className="text-sm font-medium truncate">{item.file.name}</p>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6"
                    onClick={() => removeFile(item.file)}
                    disabled={item.status === 'uploading'}
                  >
                    <X className="h-4 w-4" />
                  </Button>
                </div>
                <Progress value={item.progress} className="h-2" />
              </div>
              {item.status === 'uploading' && <Loader2 className="h-4 w-4 animate-spin text-blue-500 dark:text-blue-400" />}
              {item.status === 'completed' && <span className="text-xs text-green-600 dark:text-green-400 font-medium">Done</span>}
              {item.status === 'error' && <span className="text-xs text-red-600 dark:text-red-400 font-medium">Error</span>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
