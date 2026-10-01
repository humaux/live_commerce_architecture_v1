// Loading skeletons for the list pages (shown by each route's loading.tsx while the server reads Go). Pages that can answer a
// 404 or a permanent redirect (product, collection) deliberately have NO loading.tsx: a Suspense boundary flushes status 200 first. Pure markup, no
// data, no BFF/Go. aria-busy + a visually hidden status keep screen readers informed; the shimmer stops under
// prefers-reduced-motion (see app/shop.css).
export function GridSkeleton({ title = true }: { title?: boolean }) {
  return (
    <main className="sf-wrap" aria-busy="true">
      <p className="sr-only" role="status">
        …
      </p>
      {title && <div className="sf-skel sf-skel--title" />}
      <div className="sf-grid" aria-hidden="true">
        {Array.from({ length: 8 }, (_, i) => (
          <div className="sf-card" key={i}>
            <div className="sf-skel sf-skel--media" />
            <div className="sf-skel sf-skel--line" />
            <div className="sf-skel sf-skel--line sf-skel--short" />
          </div>
        ))}
      </div>
    </main>
  );
}
