import { useState, useEffect } from 'react';
import { Pencil, Eye, Save } from 'lucide-react';
import { MarkdownContent } from '../chat/MarkdownContent';

interface FileViewerProps {
  path: string;
  content: string;
  onSave: (content: string) => void;
  isLoading: boolean;
}

export default function FileViewer({ path, content, onSave, isLoading }: FileViewerProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState(content);

  useEffect(() => {
    setEditContent(content);
    setIsEditing(false);
  }, [content, path]);

  if (!path) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-500">
        Select a file to view
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-400">
        <div className="flex items-center gap-2">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-violet-500 border-t-transparent" />
          Loading...
        </div>
      </div>
    );
  }

  const handleSave = () => {
    onSave(editContent);
    setIsEditing(false);
  };

  return (
    <div className="glass-card flex h-full flex-col overflow-hidden">
      {/* Toolbar */}
      <div className="flex items-center justify-between border-b border-white/5 px-4 py-2">
        <span className="text-xs font-mono text-zinc-400 truncate">{path}</span>
        <div className="flex items-center gap-1">
          {isEditing && (
            <button
              onClick={handleSave}
              className="flex items-center gap-1 rounded-md bg-violet-500/20 px-2 py-1 text-xs text-violet-400 transition-colors hover:bg-violet-500/30"
            >
              <Save size={12} />
              Save
            </button>
          )}
          <button
            onClick={() => setIsEditing((prev) => !prev)}
            className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-zinc-400 transition-colors hover:bg-white/5 hover:text-zinc-300"
          >
            {isEditing ? <Eye size={12} /> : <Pencil size={12} />}
            {isEditing ? 'View' : 'Edit'}
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-4">
        {isEditing ? (
          <textarea
            value={editContent}
            onChange={(e) => setEditContent(e.target.value)}
            className="h-full w-full resize-none bg-[#0f1117] font-mono text-sm text-zinc-300 outline-none rounded-md p-3 border border-white/5"
          />
        ) : (
          <div className="prose-sm text-zinc-300">
            <MarkdownContent content={content} />
          </div>
        )}
      </div>
    </div>
  );
}
