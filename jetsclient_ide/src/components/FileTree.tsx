import { useMemo, useState } from "react";
import { fileNameOf, type WorkspaceNode } from "../api/workspace";

interface Props {
  nodes: WorkspaceNode[];
  activeFile: string | null;
  onOpen: (node: WorkspaceNode) => void;
}

/** Human-readable size. Files here run from a few bytes to a few megabytes. */
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** Keeps a node whose own label matches, or which has a surviving descendant. */
function filterTree(nodes: WorkspaceNode[], needle: string): WorkspaceNode[] {
  if (needle === "") return nodes;
  const lower = needle.toLowerCase();
  const keep = (n: WorkspaceNode): WorkspaceNode | null => {
    const children = n.children ? n.children.map(keep).filter((c): c is WorkspaceNode => c !== null) : [];
    if (n.label.toLowerCase().includes(lower) || children.length > 0) {
      return { ...n, children: children.length > 0 ? children : n.children };
    }
    return null;
  };
  return nodes.map(keep).filter((n): n is WorkspaceNode => n !== null);
}

function TreeNode({
  node,
  depth,
  activeFile,
  onOpen,
  forceOpen,
}: {
  node: WorkspaceNode;
  depth: number;
  activeFile: string | null;
  onOpen: (n: WorkspaceNode) => void;
  forceOpen: boolean;
}) {
  const [open, setOpen] = useState(depth < 1);
  const expanded = forceOpen || open;
  const isFile = node.type === "file";
  const fileName = fileNameOf(node);
  const isActive = isFile && fileName !== null && fileName === activeFile;

  if (isFile) {
    return (
      <button
        type="button"
        className={`tree-row tree-file${isActive ? " is-active" : ""}`}
        style={{ paddingLeft: `${8 + depth * 13}px` }}
        onClick={() => onOpen(node)}
        title={node.label}
      >
        <span className="tree-label">{node.label}</span>
        <span className="tree-size">{formatSize(node.size)}</span>
      </button>
    );
  }

  return (
    <div>
      <button
        type="button"
        className="tree-row tree-dir"
        style={{ paddingLeft: `${8 + depth * 13}px` }}
        onClick={() => setOpen((v) => !v)}
        aria-expanded={expanded}
      >
        <span className={`tree-caret${expanded ? " is-open" : ""}`} aria-hidden="true">
          ▸
        </span>
        <span className="tree-label">{node.label}</span>
      </button>
      {expanded && node.children && (
        <div>
          {node.children.map((child, i) => (
            <TreeNode
              key={`${child.key}:${child.label}:${i}`}
              node={child}
              depth={depth + 1}
              activeFile={activeFile}
              onOpen={onOpen}
              forceOpen={forceOpen}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function FileTree({ nodes, activeFile, onOpen }: Props) {
  const [filter, setFilter] = useState("");
  const filtered = useMemo(() => filterTree(nodes, filter.trim()), [nodes, filter]);

  return (
    <div className="tree">
      <div className="tree-filter">
        <input
          type="search"
          value={filter}
          placeholder="Filter files"
          onChange={(e) => setFilter(e.target.value)}
          aria-label="Filter files"
        />
      </div>
      <div className="tree-scroll">
        {filtered.length === 0 ? (
          <p className="tree-empty">No files match “{filter}”.</p>
        ) : (
          filtered.map((n, i) => (
            <TreeNode
              key={`${n.key}:${n.label}:${i}`}
              node={n}
              depth={0}
              activeFile={activeFile}
              onOpen={onOpen}
              // While filtering, show the matches rather than making the user
              // expand to find them.
              forceOpen={filter.trim() !== ""}
            />
          ))
        )}
      </div>
    </div>
  );
}
