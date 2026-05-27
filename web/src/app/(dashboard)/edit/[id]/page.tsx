'use client';

import { useParams } from 'next/navigation';
import { DocumentEditor } from '@/components/editor/document-editor';

export default function EditFilePage() {
  const params = useParams();
  const id = params.id as string;

  return (
    <div className="mx-auto max-w-4xl">
      <DocumentEditor fileId={id} />
    </div>
  );
}
