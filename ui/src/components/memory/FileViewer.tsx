import { useState, useEffect } from "react";
import { Pencil, Eye, Save, File } from "lucide-react";
import { MarkdownContent } from "@/components/chat/MarkdownContent";

interface FileViewerProps {
  path: string | null;
  content: string;
  loading: boolean;
  onSave: (content: string) => void;
}

export default function FileViewer({ path, content, loading, onSave }: FileViewerProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState(content);

  useEffect(() => {
    setEditContent(content);
    setIsEditing(false);
  }, [content, path]);

  if (!path) {
    return (
      <div className="flex flex-col h-full items-center justify-center gap-2">
        <File size={36} className="text-fg-3 opacity-30" />
        <span className="text-fg-3 text-xs font-mono mt-2">Select a file to view</span>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="flex items-center gap-2 text-fg-3 text-sm">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-accent border-t-transparent" />
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
    <div className="flex h-full flex-col overflow-hidden">
      {/* Toolbar */}
      <div className="flex items-center justify-between border-b border-edge px-4 py-2">
        <span className="font-mono text-xs text-fg-2 truncate">{path}</span>
        <div className="flex items-center gap-1.5 shrink-0">
          {isEditing && (
            <button
              onClick={handleSave}
              className="btn-accent !px-2.5 !py-1 !text-xs flex items-center gap-1"
            >
              <Save size={12} />
              Save
            </button>
          )}
          <button
            onClick={() => setIsEditing((prev) => !prev)}
            className="btn !px-2 !py-1 !text-xs flex items-center gap-1"
          >
            {isEditing ? <Eye size={12} /> : <Pencil size={12} />}
            {isEditing ? "View" : "Edit"}
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-4">
        {isEditing ? (
          <textarea
            value={editContent}
            onChange={(e) => setEditContent(e.target.value)}
            className="h-full w-full resize-none bg-base border border-edge rounded-md font-mono text-sm text-fg p-3 input-focus"
          />
        ) : (
          <div className="text-sm text-fg">
            <MarkdownContent content={content} />
          </div>
        )}
      </div>
    </div>
  );
}
