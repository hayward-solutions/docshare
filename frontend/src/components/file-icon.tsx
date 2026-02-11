import { 
  FileText, 
  Image as ImageIcon, 
  Music, 
  Video, 
  Folder, 
  FileCode, 
  File as FileIcon,
  FileSpreadsheet,
  FileArchive
} from 'lucide-react';

interface FileIconProps {
  mimeType: string;
  isDirectory: boolean;
  className?: string;
}

export function FileIconComponent({ mimeType, isDirectory, className }: FileIconProps) {
  if (isDirectory) {
    return <Folder className={className} fill="currentColor" />;
  }

  if (mimeType.startsWith('image/')) {
    return <ImageIcon className={className} />;
  }

  if (mimeType.startsWith('video/')) {
    return <Video className={className} />;
  }

  if (mimeType.startsWith('audio/')) {
    return <Music className={className} />;
  }

  if (mimeType === 'application/pdf') {
    return <FileText className={className} />;
  }

  if (
    mimeType.includes('spreadsheet') || 
    mimeType.includes('excel') || 
    mimeType.includes('csv')
  ) {
    return <FileSpreadsheet className={className} />;
  }

  if (
    mimeType.includes('zip') || 
    mimeType.includes('compressed') || 
    mimeType.includes('tar')
  ) {
    return <FileArchive className={className} />;
  }

  if (
    mimeType.includes('json') || 
    mimeType.includes('xml') || 
    mimeType.includes('javascript') || 
    mimeType.includes('typescript') || 
    mimeType.includes('html') || 
    mimeType.includes('css')
  ) {
    return <FileCode className={className} />;
  }

  return <FileIcon className={className} />;
}
