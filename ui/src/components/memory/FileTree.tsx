import { useState } from "react";
import {
  ChevronRight,
  ChevronDown,
  File,
  FileText,
  FolderOpen,
  Folder,
  Brain,
} from "lucide-react";
import type { MemoryNode } from "@/types";

interface FileTreeProps {
  tree: MemoryNode | null;
  selectedPath: string | null;
  onSelect: (path: string) => void;
}

function getFileIcon(name: string) {
  if (name.endsWith(".md")) return FileText;
  return File;
}

function TreeNode({
  node,
  selectedPath,
  onSelect,
  depth = 0,
}: {
  node: MemoryNode;
  selectedPath: string | null;
  onSelect: (path: string) => void;
  depth?: number;
}) {
  const [expanded, setExpanded] = useState(depth < 2);
  const isDir = node.type === "dir";
  const isActive = !isDir && node.path === selectedPath;
  const FileIcon = isDir ? (expanded ? FolderOpen : Folder) : getFileIcon(node.name);

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
        className={`flex w-full items-center gap-1.5 py-1.5 px-2 font-mono text-xs transition-colors duration-150 ${
          isActive
            ? "bg-accent-dim text-accent border-l-2 border-accent"
            : "text-fg-2 hover:bg-raised hover:text-fg border-l-2 border-transparent"
        }`}
        style={{ paddingLeft: depth * 16 + 8 }}
      >
        {isDir &&
          (expanded ? (
            <ChevronDown size={12} className="shrink-0 text-fg-3" />
          ) : (
            <ChevronRight size={12} className="shrink-0 text-fg-3" />
          ))}
        <FileIcon
          size={13}
          className={`shrink-0 ${
            isDir ? "text-accent opacity-60" : isActive ? "text-accent" : "text-fg-3"
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
  if (!tree || !tree.children || tree.children.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-1 px-4">
        <Brain size={36} className="text-fg-3 opacity-30" />
        <span className="text-fg-3 text-xs font-mono mt-3">No memory files</span>
        <span className="text-fg-3 text-[10px] opacity-60 text-center leading-relaxed">
          Files appear as the agent builds memory
        </span>
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
