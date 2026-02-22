import { useState, useEffect } from 'react';
import { FilePlus } from 'lucide-react';
import FileTree from './FileTree';
import FileViewer from './FileViewer';
import { getMemoryTree, getMemoryFile, putMemoryFile } from '../../lib/api';
import type { MemoryNode } from '../../types';

export default function MemoryTab() {
  const [tree, setTree] = useState<MemoryNode | null>(null);
  const [selectedFile, setSelectedFile] = useState('');
  const [fileContent, setFileContent] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    getMemoryTree()
      .then(setTree)
      .catch(() => {});
  }, []);

  const handleSelect = async (path: string) => {
    setSelectedFile(path);
    setIsLoading(true);
    try {
      const data = await getMemoryFile(path);
      setFileContent(data.content);
    } catch {
      setFileContent('');
    } finally {
      setIsLoading(false);
    }
  };

  const handleSave = async (content: string) => {
    try {
      await putMemoryFile(selectedFile, content);
      setFileContent(content);
    } catch {
      // save failed silently
    }
  };

  const handleNewFile = async () => {
    const name = window.prompt('File name (e.g. notes.md):');
    if (!name) return;
    const path = name.startsWith('/') ? name.slice(1) : name;
    try {
      await putMemoryFile(path, '');
      const refreshed = await getMemoryTree();
      setTree(refreshed);
      setSelectedFile(path);
      setFileContent('');
    } catch {
      // create failed
    }
  };

  return (
    <div className="flex h-full flex-col bg-[#08090d]">
      {/* Top toolbar */}
      <div className="flex items-center justify-between border-b border-white/5 px-4 py-2">
        <span className="text-sm font-medium text-zinc-300">Memory Explorer</span>
        <button
          onClick={handleNewFile}
          className="flex items-center gap-1 rounded-md bg-violet-500/15 px-2.5 py-1 text-xs text-violet-400 transition-colors hover:bg-violet-500/25"
        >
          <FilePlus size={13} />
          New File
        </button>
      </div>

      {/* Split pane */}
      <div className="flex flex-1 overflow-hidden">
        {/* Left panel: file tree */}
        <div className="w-60 shrink-0 overflow-y-auto border-r border-white/5 bg-[#0b0c12]">
          {tree ? (
            <FileTree tree={tree} selectedPath={selectedFile} onSelect={handleSelect} />
          ) : (
            <div className="flex items-center justify-center h-32 text-xs text-zinc-500">
              Loading...
            </div>
          )}
        </div>

        {/* Right panel: file viewer */}
        <div className="flex-1 overflow-hidden">
          <FileViewer
            path={selectedFile}
            content={fileContent}
            onSave={handleSave}
            isLoading={isLoading}
          />
        </div>
      </div>
    </div>
  );
}
