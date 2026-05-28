'use client';

import { ReactNode, useEffect, useRef, useState } from 'react';
import { Editor } from '@tiptap/react';
import {
  Bold,
  Italic,
  Strikethrough,
  Code,
  Heading1,
  Heading2,
  Heading3,
  List,
  ListOrdered,
  ListChecks,
  Quote,
  Code2,
  Minus,
  Link as LinkIcon,
  Image as ImageIcon,
  Table as TableIcon,
  Undo,
  Redo,
  ChevronDown,
} from 'lucide-react';
import { fileToDataURI, MAX_CONTENT_BYTES, markdownByteLength } from '@/lib/editor-images';
import { Button } from '@/components/ui/button';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { LinkDialog } from './link-dialog';

interface ToolbarButtonProps {
  onClick: () => void;
  disabled?: boolean;
  active?: boolean;
  tooltip: string;
  children: ReactNode;
}

function ToolbarButton({ onClick, disabled, active, tooltip, children }: ToolbarButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          disabled={disabled}
          aria-label={tooltip}
          aria-pressed={active}
          onMouseDown={(e) => e.preventDefault()}
          onClick={onClick}
          className={cn(
            'h-8 w-8',
            active && 'bg-accent text-accent-foreground',
          )}
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{tooltip}</TooltipContent>
    </Tooltip>
  );
}

function ToolbarDivider() {
  return <div className="mx-1 h-6 w-px bg-border" aria-hidden />;
}

interface EditorToolbarProps {
  editor: Editor;
  disabled?: boolean;
}

export function EditorToolbar({ editor, disabled = false }: EditorToolbarProps) {
  const [linkDialogOpen, setLinkDialogOpen] = useState(false);
  const imageInputRef = useRef<HTMLInputElement | null>(null);

  const handleImagePick = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    // Compute how much room is left in the 5 MiB save cap before letting
    // the image through. Reading the markdown serialization (not the
    // tiptap JSON tree) keeps the budget aligned with what the backend
    // will measure.
    const storage = editor.storage as { markdown?: { getMarkdown: () => string } };
    const currentBytes = markdownByteLength(storage.markdown?.getMarkdown() ?? editor.getText());
    const remaining = Math.max(0, MAX_CONTENT_BYTES - currentBytes);
    const dataUri = await fileToDataURI(file, remaining);
    if (!dataUri) return;
    editor.chain().focus().setImage({ src: dataUri }).run();
  };

  // Slash menu "Image" item fires this event since it can't trigger our
  // hidden file input directly from inside the ProseMirror command.
  useEffect(() => {
    const handler = () => imageInputRef.current?.click();
    window.addEventListener('docshare:open-image-picker', handler);
    return () => window.removeEventListener('docshare:open-image-picker', handler);
  }, []);

  const activeHeading = editor.isActive('heading', { level: 1 })
    ? 'H1'
    : editor.isActive('heading', { level: 2 })
      ? 'H2'
      : editor.isActive('heading', { level: 3 })
        ? 'H3'
        : 'Text';

  const linkAttrs = editor.getAttributes('link') as { href?: string; target?: string };
  const hasExistingLink = editor.isActive('link');

  const openLinkDialog = () => setLinkDialogOpen(true);

  const handleLinkSubmit = ({ href, openInNewTab }: { href: string; openInNewTab: boolean }) => {
    const chain = editor.chain().focus().extendMarkRange('link');
    chain.setLink({ href, target: openInNewTab ? '_blank' : null }).run();
    setLinkDialogOpen(false);
  };

  const handleLinkRemove = () => {
    editor.chain().focus().extendMarkRange('link').unsetLink().run();
  };

  return (
    <TooltipProvider delayDuration={250}>
      <div
        className={cn(
          'flex flex-wrap items-center gap-0.5 rounded-md border bg-card p-1',
          disabled && 'opacity-60',
        )}
        role="toolbar"
        aria-label="Document formatting"
      >
        <DropdownMenu>
          <Tooltip>
            <TooltipTrigger asChild>
              <DropdownMenuTrigger asChild disabled={disabled}>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  disabled={disabled}
                  className="h-8 min-w-[5.5rem] justify-between gap-1.5 px-2"
                >
                  <span className="text-xs font-medium">{activeHeading}</span>
                  <ChevronDown className="h-3 w-3 opacity-60" />
                </Button>
              </DropdownMenuTrigger>
            </TooltipTrigger>
            <TooltipContent side="bottom">Text style</TooltipContent>
          </Tooltip>
          <DropdownMenuContent align="start" className="w-40">
            <DropdownMenuItem onClick={() => editor.chain().focus().setParagraph().run()}>
              <span className="text-sm">Paragraph</span>
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()}>
              <Heading1 className="mr-2 h-4 w-4" /> Heading 1
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}>
              <Heading2 className="mr-2 h-4 w-4" /> Heading 2
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}>
              <Heading3 className="mr-2 h-4 w-4" /> Heading 3
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        <ToolbarDivider />

        <ToolbarButton
          tooltip="Bold (⌘B)"
          active={editor.isActive('bold')}
          onClick={() => editor.chain().focus().toggleBold().run()}
        >
          <Bold className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Italic (⌘I)"
          active={editor.isActive('italic')}
          onClick={() => editor.chain().focus().toggleItalic().run()}
        >
          <Italic className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Strikethrough"
          active={editor.isActive('strike')}
          onClick={() => editor.chain().focus().toggleStrike().run()}
        >
          <Strikethrough className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Inline code"
          active={editor.isActive('code')}
          onClick={() => editor.chain().focus().toggleCode().run()}
        >
          <Code className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Link"
          active={editor.isActive('link')}
          onClick={openLinkDialog}
        >
          <LinkIcon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Insert image (≤2 MiB)"
          onClick={() => imageInputRef.current?.click()}
        >
          <ImageIcon className="h-4 w-4" />
        </ToolbarButton>

        <ToolbarDivider />

        <ToolbarButton
          tooltip="Bullet list"
          active={editor.isActive('bulletList')}
          onClick={() => editor.chain().focus().toggleBulletList().run()}
        >
          <List className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Numbered list"
          active={editor.isActive('orderedList')}
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
        >
          <ListOrdered className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Task list"
          active={editor.isActive('taskList')}
          onClick={() => editor.chain().focus().toggleTaskList().run()}
        >
          <ListChecks className="h-4 w-4" />
        </ToolbarButton>

        <ToolbarDivider />

        <ToolbarButton
          tooltip="Blockquote"
          active={editor.isActive('blockquote')}
          onClick={() => editor.chain().focus().toggleBlockquote().run()}
        >
          <Quote className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Code block"
          active={editor.isActive('codeBlock')}
          onClick={() => editor.chain().focus().toggleCodeBlock().run()}
        >
          <Code2 className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Horizontal rule"
          onClick={() => editor.chain().focus().setHorizontalRule().run()}
        >
          <Minus className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Insert table"
          onClick={() =>
            editor
              .chain()
              .focus()
              .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
              .run()
          }
        >
          <TableIcon className="h-4 w-4" />
        </ToolbarButton>

        <ToolbarDivider />

        <ToolbarButton
          tooltip="Undo (⌘Z)"
          disabled={!editor.can().undo()}
          onClick={() => editor.chain().focus().undo().run()}
        >
          <Undo className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton
          tooltip="Redo (⌘⇧Z)"
          disabled={!editor.can().redo()}
          onClick={() => editor.chain().focus().redo().run()}
        >
          <Redo className="h-4 w-4" />
        </ToolbarButton>
      </div>
      <LinkDialog
        open={linkDialogOpen}
        onOpenChange={setLinkDialogOpen}
        initialHref={linkAttrs.href ?? ''}
        initialOpenInNewTab={linkAttrs.target === '_blank'}
        hasExistingLink={hasExistingLink}
        onSubmit={handleLinkSubmit}
        onRemove={handleLinkRemove}
      />
      <input
        ref={imageInputRef}
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp,image/svg+xml"
        className="hidden"
        onChange={handleImagePick}
      />
    </TooltipProvider>
  );
}
