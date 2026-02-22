import { useState, useEffect } from "react";
import { FilePlus } from "lucide-react";
import FileTree from "@/components/memory/FileTree";
import FileViewer from "@/components/memory/FileViewer";
import { getMemoryTree, getMemoryFile, putMemoryFile } from "@/lib/api";
import type { MemoryNode } from "@/types";

export default function MemoryTab() {
  const [tree, setTree] = useState<MemoryNode | null>(null);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [fileContent, setFileContent] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    getMemoryTree()
      .then(setTree)
      .catch(() => {});
  }, []);

  const handleSelect = async (path: string) => {
    setSelectedFile(path);
    setLoading(true);
    try {
      const data = await getMemoryFile(path);
      setFileContent(data.content);
    } catch {
      setFileContent("");
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async (content: string) => {
    if (!selectedFile) return;
    try {
      await putMemoryFile(selectedFile, content);
      setFileContent(content);
    } catch {
      // save failed silently
    }
  };

  const handleNewFile = async () => {
    const name = window.prompt("File name (e.g. notes.md):");
    if (!name) return;
    const path = name.startsWith("/") ? name.slice(1) : name;
    try {
      await putMemoryFile(path, "");
      const refreshed = await getMemoryTree();
      setTree(refreshed);
      setSelectedFile(path);
      setFileContent("");
    } catch {
      // create failed silently
    }
  };

  return (
    <div className="flex h-full flex-col bg-base">
      {/* Toolbar */}
      <div className="flex items-center justify-between border-b border-edge px-4 py-3">
        <span className="section-label">Memory Explorer</span>
        <button
          onClick={handleNewFile}
          className="btn flex items-center gap-1.5"
        >
          <FilePlus size={13} />
          New File
        </button>
      </div>

      {/* Split pane */}
      <div className="flex flex-1 overflow-hidden">
        {/* File tree sidebar */}
        <div className="w-56 shrink-0 overflow-y-auto border-r border-edge bg-surface">
          {tree ? (
            <FileTree tree={tree} selectedPath={selectedFile} onSelect={handleSelect} />
          ) : (
            <div className="flex items-center justify-center h-32">
              <div className="flex items-center gap-2">
                <div className="h-3 w-3 animate-spin rounded-full border-2 border-accent border-t-transparent" />
                <span className="text-xs font-mono text-fg-3">Loading...</span>
              </div>
            </div>
          )}
        </div>

        {/* File viewer */}
        <div className="flex-1 overflow-hidden">
          <FileViewer
            path={selectedFile}
            content={fileContent}
            loading={loading}
            onSave={handleSave}
          />
        </div>
      </div>
    </div>
  );
}
