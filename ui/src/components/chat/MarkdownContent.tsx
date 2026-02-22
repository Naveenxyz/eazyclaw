import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import type { Components } from "react-markdown";

interface MarkdownContentProps {
  content: string;
}

const components: Components = {
  code({ className, children, ...props }) {
    const isInline = !className;
    if (isInline) {
      return (
        <code
          className="rounded bg-accent-dim px-1.5 py-0.5 text-xs font-mono text-accent"
          {...props}
        >
          {children}
        </code>
      );
    }
    return (
      <code className={`${className ?? ""} font-mono`} {...props}>
        {children}
      </code>
    );
  },

  pre({ children }) {
    return (
      <pre className="my-2 overflow-x-auto rounded-md border border-edge bg-base p-3 text-xs">
        {children}
      </pre>
    );
  },

  p({ children }) {
    return (
      <p className="mb-2 text-sm text-fg leading-relaxed last:mb-0">
        {children}
      </p>
    );
  },

  ul({ children }) {
    return (
      <ul className="mb-2 list-disc pl-5 text-sm space-y-1">{children}</ul>
    );
  },

  ol({ children }) {
    return (
      <ol className="mb-2 list-decimal pl-5 text-sm space-y-1">{children}</ol>
    );
  },

  a({ href, children }) {
    return (
      <a
        href={href}
        className="text-accent underline hover:text-accent-strong"
        target="_blank"
        rel="noopener noreferrer"
      >
        {children}
      </a>
    );
  },

  h1({ children }) {
    return (
      <h1 className="mb-3 text-xl font-display font-bold text-fg">
        {children}
      </h1>
    );
  },

  h2({ children }) {
    return (
      <h2 className="mb-2 text-lg font-display font-bold text-fg">
        {children}
      </h2>
    );
  },

  h3({ children }) {
    return (
      <h3 className="mb-2 text-base font-display font-bold text-fg">
        {children}
      </h3>
    );
  },

  blockquote({ children }) {
    return (
      <blockquote className="mb-2 border-l-2 border-fg-3 pl-3 italic text-fg-2">
        {children}
      </blockquote>
    );
  },

  table({ children }) {
    return (
      <div className="mb-2 overflow-x-auto">
        <table className="w-full text-xs border-collapse">{children}</table>
      </div>
    );
  },

  th({ children }) {
    return (
      <th className="bg-raised px-3 py-1.5 text-left font-semibold text-fg border-b border-edge">
        {children}
      </th>
    );
  },

  td({ children }) {
    return (
      <td className="px-3 py-1.5 text-fg-2 border-b border-edge">
        {children}
      </td>
    );
  },
};

export function MarkdownContent({ content }: MarkdownContentProps) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      rehypePlugins={[rehypeHighlight]}
      components={components}
    >
      {content}
    </ReactMarkdown>
  );
}
