import { useState } from 'react';
import { ChevronRight, ChevronDown, File, Folder, FileText } from 'lucide-react';
import type { MemoryNode } from '../../types';

interface FileTreeProps {
  tree: MemoryNode;
  selectedPath: string;
  onSelect: (path: string) => void;
}

function getFileIcon(name: string) {
  if (name.endsWith('.md')) return FileText;
  return File;
}

function TreeNode({
  node,
  selectedPath,
  onSelect,
  depth = 0,
}: {
  node: MemoryNode;
  selectedPath: string;
  onSelect: (path: string) => void;
  depth?: number;
}) {
  const [expanded, setExpanded] = useState(depth < 2);
  const isDir = node.type === 'dir';
  const isActive = node.path === selectedPath;
  const Icon = isDir ? Folder : getFileIcon(node.name);

  const handleClick = () => {
    if (isDir) {
      setExpanded((prev) => !prev);
    } else {
      onSelect(node.path);
    }
  };

  return (
    <div>
      <button
        onClick={handleClick}
        className={`flex w-full items-center gap-1.5 rounded-md px-2 py-1 text-sm font-mono transition-colors ${
          isActive
            ? 'bg-violet-500/10 text-violet-400 border-l-2 border-violet-500'
            : 'text-zinc-400 hover:bg-white/5 border-l-2 border-transparent'
        }`}
        style={{ paddingLeft: depth * 16 + 8 }}
      >
        {isDir && (
          expanded ? (
            <ChevronDown size={14} className="shrink-0 text-zinc-500" />
          ) : (
            <ChevronRight size={14} className="shrink-0 text-zinc-500" />
          )
        )}
        <Icon
          size={14}
          className={`shrink-0 ${
            isDir ? 'text-violet-400/70' : isActive ? 'text-violet-400' : 'text-zinc-500'
          }`}
        />
        <span className="truncate">{node.name}</span>
      </button>
      {isDir && expanded && node.children && (
        <div>
          {node.children.map((child) => (
            <TreeNode
              key={child.path}
              node={child}
              selectedPath={selectedPath}
              onSelect={onSelect}
              depth={depth + 1}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export default function FileTree({ tree, selectedPath, onSelect }: FileTreeProps) {
  if (!tree.children || tree.children.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-sm text-zinc-500">
        No files
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-0.5 py-2">
      {tree.children.map((child) => (
        <TreeNode
          key={child.path}
          node={child}
          selectedPath={selectedPath}
          onSelect={onSelect}
        />
      ))}
    </div>
  );
}
