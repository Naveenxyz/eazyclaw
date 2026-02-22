import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";

interface MarkdownContentProps {
  content: string;
}

export function MarkdownContent({ content }: MarkdownContentProps) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      rehypePlugins={[rehypeHighlight]}
      children={content}
      components={{
        code({ className, children, ...props }) {
          const isInline = !className;
          if (isInline) {
            return (
              <code
                className="rounded bg-zinc-800 px-1.5 py-0.5 text-sm font-mono text-violet-300"
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
            <pre className="my-2 overflow-x-auto rounded-md bg-zinc-900 p-3 text-sm">
              {children}
            </pre>
          );
        },
        p({ children }) {
          return <p className="mb-2 last:mb-0">{children}</p>;
        },
        ul({ children }) {
          return <ul className="mb-2 ml-4 list-disc">{children}</ul>;
        },
        ol({ children }) {
          return <ol className="mb-2 ml-4 list-decimal">{children}</ol>;
        },
        li({ children }) {
          return <li className="mb-1">{children}</li>;
        },
        a({ href, children }) {
          return (
            <a
              href={href}
              className="text-violet-400 underline hover:text-violet-300"
              target="_blank"
              rel="noopener noreferrer"
            >
              {children}
            </a>
          );
        },
        h1({ children }) {
          return <h1 className="mb-2 text-lg font-bold">{children}</h1>;
        },
        h2({ children }) {
          return <h2 className="mb-2 text-base font-bold">{children}</h2>;
        },
        h3({ children }) {
          return <h3 className="mb-2 text-sm font-bold">{children}</h3>;
        },
        blockquote({ children }) {
          return (
            <blockquote className="mb-2 border-l-2 border-zinc-600 pl-3 italic text-zinc-400">
              {children}
            </blockquote>
          );
        },
        table({ children }) {
          return (
            <div className="mb-2 overflow-x-auto">
              <table className="min-w-full border-collapse text-sm">
                {children}
              </table>
            </div>
          );
        },
        th({ children }) {
          return (
            <th className="border border-zinc-700 bg-zinc-800 px-3 py-1.5 text-left font-semibold">
              {children}
            </th>
          );
        },
        td({ children }) {
          return (
            <td className="border border-zinc-700 px-3 py-1.5">{children}</td>
          );
        },
      }}
    />
  );
}
