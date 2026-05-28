'use client';

import { Extension, type Range } from '@tiptap/core';
import { ReactRenderer, type Editor } from '@tiptap/react';
import Suggestion, { type SuggestionProps } from '@tiptap/suggestion';
import { PluginKey } from '@tiptap/pm/state';
import {
  forwardRef,
  useImperativeHandle,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
} from 'react';
import {
  Heading1,
  Heading2,
  Heading3,
  Image as ImageIcon,
  List,
  ListOrdered,
  ListChecks,
  Quote,
  Code2,
  Minus,
  Table as TableIcon,
  Type,
  type LucideIcon,
} from 'lucide-react';
import { cn } from '@/lib/utils';

interface SlashCommandItem {
  title: string;
  description: string;
  keywords: string[];
  icon: LucideIcon;
  command: (args: { editor: Editor; range: Range }) => void;
}

const COMMANDS: SlashCommandItem[] = [
  {
    title: 'Paragraph',
    description: 'Plain text block',
    keywords: ['p', 'text', 'paragraph'],
    icon: Type,
    command: ({ editor, range }) => {
      editor.chain().focus().deleteRange(range).setParagraph().run();
    },
  },
  {
    title: 'Heading 1',
    description: 'Large section heading',
    keywords: ['h1', 'heading', 'title'],
    icon: Heading1,
    command: ({ editor, range }) => {
      editor.chain().focus().deleteRange(range).setNode('heading', { level: 1 }).run();
    },
  },
  {
    title: 'Heading 2',
    description: 'Medium section heading',
    keywords: ['h2', 'heading', 'subtitle'],
    icon: Heading2,
    command: ({ editor, range }) => {
      editor.chain().focus().deleteRange(range).setNode('heading', { level: 2 }).run();
    },
  },
  {
    title: 'Heading 3',
    description: 'Small section heading',
    keywords: ['h3', 'heading'],
    icon: Heading3,
    command: ({ editor, range }) => {
      editor.chain().focus().deleteRange(range).setNode('heading', { level: 3 }).run();
    },
  },
  {
    title: 'Bullet list',
    description: 'Unordered list',
    keywords: ['ul', 'list', 'bullet'],
    icon: List,
    command: ({ editor, range }) => {
      editor.chain().focus().deleteRange(range).toggleBulletList().run();
    },
  },
  {
    title: 'Numbered list',
    description: 'Ordered list',
    keywords: ['ol', 'list', 'numbered', 'ordered'],
    icon: ListOrdered,
    command: ({ editor, range }) => {
      editor.chain().focus().deleteRange(range).toggleOrderedList().run();
    },
  },
  {
    title: 'Task list',
    description: 'Checkable to-do items',
    keywords: ['todo', 'task', 'checkbox', 'check'],
    icon: ListChecks,
    command: ({ editor, range }) => {
      editor.chain().focus().deleteRange(range).toggleTaskList().run();
    },
  },
  {
    title: 'Quote',
    description: 'Blockquote',
    keywords: ['quote', 'blockquote', 'citation'],
    icon: Quote,
    command: ({ editor, range }) => {
      editor.chain().focus().deleteRange(range).toggleBlockquote().run();
    },
  },
  {
    title: 'Code block',
    description: 'Syntax-highlighted code',
    keywords: ['code', 'codeblock', 'pre'],
    icon: Code2,
    command: ({ editor, range }) => {
      editor.chain().focus().deleteRange(range).setCodeBlock().run();
    },
  },
  {
    title: 'Divider',
    description: 'Horizontal rule',
    keywords: ['hr', 'rule', 'divider', 'separator'],
    icon: Minus,
    command: ({ editor, range }) => {
      editor.chain().focus().deleteRange(range).setHorizontalRule().run();
    },
  },
  {
    title: 'Table',
    description: '3×3 table with header row',
    keywords: ['table', 'grid'],
    icon: TableIcon,
    command: ({ editor, range }) => {
      editor
        .chain()
        .focus()
        .deleteRange(range)
        .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
        .run();
    },
  },
  {
    title: 'Image',
    description: 'Upload an image (≤2 MiB)',
    keywords: ['image', 'photo', 'picture', 'img'],
    icon: ImageIcon,
    command: ({ editor, range }) => {
      // Drop the "/image" trigger first, then dispatch a global event that
      // the toolbar listens for to open its hidden file input. Doing it
      // here keeps the slash extension stateless and avoids passing React
      // refs through ProseMirror.
      editor.chain().focus().deleteRange(range).run();
      window.dispatchEvent(new CustomEvent('docshare:open-image-picker'));
    },
  },
];

function filterCommands(query: string): SlashCommandItem[] {
  if (!query) return COMMANDS;
  const q = query.toLowerCase();
  return COMMANDS.filter((cmd) => {
    if (cmd.title.toLowerCase().includes(q)) return true;
    return cmd.keywords.some((k) => k.startsWith(q));
  });
}

interface SlashMenuHandle {
  onKeyDown: (event: KeyboardEvent) => boolean;
}

interface SlashMenuProps extends SuggestionProps<SlashCommandItem> {
  items: SlashCommandItem[];
}

const SlashMenu = forwardRef<SlashMenuHandle, SlashMenuProps>(function SlashMenu(
  { items, command },
  ref,
) {
  const [active, setActive] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  // Reset the highlighted row when the filtered list changes, via React 19
  // render-phase compare instead of useEffect+setState.
  const [prevItems, setPrevItems] = useState(items);
  if (items !== prevItems) {
    setPrevItems(items);
    setActive(0);
  }

  // Keep the highlighted row scrolled into view as the user navigates.
  useLayoutEffect(() => {
    const el = listRef.current?.querySelector<HTMLElement>(`[data-index="${active}"]`);
    el?.scrollIntoView({ block: 'nearest' });
  }, [active]);

  useImperativeHandle(ref, () => ({
    onKeyDown: (event: KeyboardEvent) => {
      if (event.key === 'ArrowDown') {
        setActive((a) => (a + 1) % Math.max(items.length, 1));
        return true;
      }
      if (event.key === 'ArrowUp') {
        setActive((a) => (a - 1 + items.length) % Math.max(items.length, 1));
        return true;
      }
      if (event.key === 'Enter') {
        const item = items[active];
        if (item) command(item);
        return true;
      }
      return false;
    },
  }));

  if (items.length === 0) {
    return (
      <div className="z-50 w-64 rounded-md border bg-popover p-2 text-sm text-muted-foreground shadow-md">
        No matching blocks
      </div>
    );
  }

  return (
    <div
      ref={listRef}
      className="z-50 w-64 overflow-y-auto rounded-md border bg-popover p-1 text-popover-foreground shadow-md"
      style={{ maxHeight: 320 }}
      role="listbox"
    >
      {items.map((item, index) => {
        const Icon = item.icon;
        const isActive = index === active;
        return (
          <button
            key={item.title}
            data-index={index}
            type="button"
            role="option"
            aria-selected={isActive}
            onMouseEnter={() => setActive(index)}
            onMouseDown={(e) => {
              // Mouse down before click so we don't blur the editor first.
              e.preventDefault();
              command(item);
            }}
            className={cn(
              'flex w-full items-center gap-3 rounded-sm px-2 py-1.5 text-left text-sm transition-colors',
              isActive ? 'bg-accent text-accent-foreground' : 'text-foreground',
            )}
          >
            <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md border bg-background">
              <Icon className="h-4 w-4" />
            </span>
            <span className="flex min-w-0 flex-col">
              <span className="font-medium">{item.title}</span>
              <span className="truncate text-xs text-muted-foreground">{item.description}</span>
            </span>
          </button>
        );
      })}
    </div>
  );
});

interface PopupHandle {
  setPosition: (rect: DOMRect) => void;
  destroy: () => void;
  setVisible: (visible: boolean) => void;
}

function createPopup(component: ReactRenderer<SlashMenuHandle, SlashMenuProps>): PopupHandle {
  const wrapper = document.createElement('div');
  wrapper.style.position = 'absolute';
  wrapper.style.top = '0';
  wrapper.style.left = '0';
  wrapper.style.pointerEvents = 'none';
  wrapper.style.zIndex = '50';

  const inner = document.createElement('div');
  inner.style.position = 'absolute';
  inner.style.pointerEvents = 'auto';
  wrapper.appendChild(inner);

  // ReactRenderer renders into component.element. Move it into our inner
  // container so we control absolute positioning relative to the document.
  if (component.element) {
    inner.appendChild(component.element);
  }
  document.body.appendChild(wrapper);

  return {
    setPosition: (rect) => {
      const margin = 6;
      const menuHeight = inner.firstElementChild?.getBoundingClientRect().height ?? 300;
      const spaceBelow = window.innerHeight - rect.bottom;
      const placeAbove = spaceBelow < menuHeight + margin && rect.top > menuHeight + margin;
      const top = placeAbove ? rect.top + window.scrollY - menuHeight - margin : rect.bottom + window.scrollY + margin;
      const left = Math.min(rect.left + window.scrollX, window.scrollX + window.innerWidth - 280);
      inner.style.top = `${top}px`;
      inner.style.left = `${left}px`;
    },
    setVisible: (visible) => {
      wrapper.style.display = visible ? 'block' : 'none';
    },
    destroy: () => {
      wrapper.remove();
    },
  };
}

export const SlashCommandExtension = Extension.create({
  name: 'slashCommand',

  addOptions() {
    return {
      suggestion: {
        char: '/',
        startOfLine: false,
        allowSpaces: false,
        command: ({ editor, range, props }: { editor: Editor; range: Range; props: SlashCommandItem }) => {
          props.command({ editor, range });
        },
        items: ({ query }: { query: string }) => filterCommands(query),
      },
    };
  },

  addProseMirrorPlugins() {
    return [
      Suggestion<SlashCommandItem>({
        editor: this.editor,
        pluginKey: new PluginKey('slashCommand'),
        ...this.options.suggestion,
        render: () => {
          let component: ReactRenderer<SlashMenuHandle, SlashMenuProps>;
          let popup: PopupHandle | null = null;

          return {
            onStart: (props) => {
              component = new ReactRenderer<SlashMenuHandle, SlashMenuProps>(SlashMenu, {
                props,
                editor: props.editor,
              });
              popup = createPopup(component);
              const rect = props.clientRect?.();
              if (rect) popup.setPosition(rect);
            },
            onUpdate: (props) => {
              component.updateProps(props);
              const rect = props.clientRect?.();
              if (rect && popup) popup.setPosition(rect);
            },
            onKeyDown: (props: { event: KeyboardEvent | ReactKeyboardEvent }) => {
              if (props.event.key === 'Escape') {
                popup?.setVisible(false);
                return true;
              }
              return component.ref?.onKeyDown(props.event as KeyboardEvent) ?? false;
            },
            onExit: () => {
              popup?.destroy();
              popup = null;
              component?.destroy();
            },
          };
        },
      }),
    ];
  },
});
