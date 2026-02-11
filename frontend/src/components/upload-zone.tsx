'use client';

import { useCallback, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { Upload, X, File as FileIcon, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { apiMethods } from '@/lib/api';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';

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
  const [isUploading, setIsUploading] = useState(false);

  const uploadFileToServer = useCallback(async (uploadFile: UploadingFile) => {
    setUploadingFiles(prev => prev.map(f => 
      f.file === uploadFile.file ? { ...f, status: 'uploading' } : f
    ));

    const formData = new FormData();
    formData.append('file', uploadFile.file);
    if (parentID) {
      formData.append('parentID', parentID);
    }

    try {
      const interval = setInterval(() => {
        setUploadingFiles(prev => prev.map(f => 
          f.file === uploadFile.file ? { ...f, progress: Math.min(f.progress + 10, 90) } : f
        ));
      }, 100);

      await apiMethods.upload('/api/files/upload', formData);
      
      clearInterval(interval);
      setUploadingFiles(prev => prev.map(f => 
        f.file === uploadFile.file ? { ...f, progress: 100, status: 'completed' } : f
      ));
      toast.success(`Uploaded ${uploadFile.file.name}`);
    } catch (error) {
      console.error(error);
      setUploadingFiles(prev => prev.map(f => 
        f.file === uploadFile.file ? { ...f, status: 'error' } : f
      ));
      toast.error(`Failed to upload ${uploadFile.file.name}`);
    }
  }, [parentID]);

  const onDrop = useCallback(async (acceptedFiles: File[]) => {
    const newFiles = acceptedFiles.map(file => ({
      file,
      progress: 0,
      status: 'pending' as const
    }));

    setUploadingFiles(prev => [...prev, ...newFiles]);
    setIsUploading(true);

    for (const uploadFile of newFiles) {
      await uploadFileToServer(uploadFile);
    }
    
    setIsUploading(false);
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
          isDragActive ? "border-primary bg-primary/5" : "border-slate-200 hover:border-primary/50"
        )}
      >
        <input {...getInputProps()} />
        <div className="flex flex-col items-center gap-2 text-slate-500">
          <Upload className="h-8 w-8" />
          <p className="text-sm font-medium">
            {isDragActive ? "Drop files here" : "Drag & drop files here, or click to select"}
          </p>
        </div>
      </div>

      {uploadingFiles.length > 0 && (
        <div className="space-y-2">
          {uploadingFiles.map((item) => (
            <div key={item.file.name + item.file.size} className="flex items-center gap-3 rounded-lg border p-3 bg-white">
              <FileIcon className="h-8 w-8 text-blue-500" />
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
              {item.status === 'uploading' && <Loader2 className="h-4 w-4 animate-spin text-blue-500" />}
              {item.status === 'completed' && <span className="text-xs text-green-600 font-medium">Done</span>}
              {item.status === 'error' && <span className="text-xs text-red-600 font-medium">Error</span>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
