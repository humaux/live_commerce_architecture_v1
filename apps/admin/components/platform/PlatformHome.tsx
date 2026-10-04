import { companyConfig, type PlatformLocale } from "../../lib/company";
import { platformCopy } from "../../lib/platform-copy";
import exampleShirt from "./example-shirt.jpg";

function LineIcon({ kind }: { kind: number }) {
  return (
    <svg
      viewBox="0 0 32 32"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      aria-hidden="true"
    >
      {kind === 0 ? (
        <>
          <path d="M5 6h22v17H12l-7 5z" />
          <path d="M10 14h2m3 0h2m3 0h2" />
        </>
      ) : kind === 1 ? (
        <>
          <path d="M6 10h20l2 18H4z" />
          <path d="M11 13V7a5 5 0 0 1 10 0v6" />
        </>
      ) : (
        <>
          <path d="M8 3h12l6 6v20H8zM20 3v7h6M12 16h10M12 21h10" />
        </>
      )}
    </svg>
  );
}
function Shirt() {
  return (
    <img
      className="ps-shirt"
      src={exampleShirt.src}
      width={80}
      height={80}
      alt=""
    />
  );
}
export function PlatformHome({ locale }: { locale: PlatformLocale }) {
  const c = platformCopy[locale];
  return (
    <>
      <section className="ps-hero" aria-labelledby="ps-headline">
        <div>
          <h1 id="ps-headline">
            {c.title[0]}
            <br />
            {c.title[1]}
          </h1>
          <p className="ps-intro">{c.intro}</p>
          <a
            className="ps-primary"
            href={`${companyConfig().adminOrigin}/${locale}/signup`}
          >
            {c.signup}
            <span aria-hidden="true">→</span>
          </a>
        </div>
        <ul className="ps-features">
          {c.features.map(([title, body], i) => (
            <li key={title}>
              <LineIcon kind={i} />
              <div>
                <h2>{title}</h2>
                <p>{body}</p>
              </div>
            </li>
          ))}
        </ul>
      </section>
      <section className="ps-workflow" aria-labelledby="ps-flow-title">
        <h2 id="ps-flow-title">{c.flowTitle}</h2>
        <p className="ps-disclaimer">{c.disclaimer}</p>
        <ol className="ps-steps">
          {c.steps.map(([title, body], i) => (
            <li className="ps-step" key={title}>
              <div className="ps-step-title">
                <span className="ps-number">{i + 1}</span>
                <div>
                  <h3>{title}</h3>
                  <p>{body}</p>
                </div>
              </div>
              <div
                className="ps-preview"
                aria-label={`${title} — ${c.example}`}
              >
                <div className="ps-preview-top">
                  <strong>
                    {i === 0
                      ? "Facebook"
                      : i === 1
                        ? c.steps[1][0]
                        : c.steps[2][0]}
                  </strong>
                  <small>{c.example}</small>
                </div>
                {i === 0 ? (
                  <div className="ps-comment">
                    <span className="ps-avatar" aria-hidden="true">
                      A
                    </span>
                    <div>
                      <strong>Alex</strong>
                      <p>{c.comment}</p>
                    </div>
                  </div>
                ) : (
                  <div className="ps-product">
                    <Shirt />
                    <div>
                      <strong>{c.product}</strong>
                      <p>{c.selected}</p>
                      <b>NT$590</b>
                    </div>
                  </div>
                )}
                {i === 0 ? (
                  <div className="ps-comment ps-comment-secondary">
                    <span className="ps-avatar" aria-hidden="true">
                      B
                    </span>
                    <div>
                      <strong>Buyer</strong>
                      <p>{c.comment}</p>
                    </div>
                  </div>
                ) : i === 1 ? (
                  <div className="ps-summary">
                    <span>{c.order}</span>
                    <strong>NT$590</strong>
                  </div>
                ) : (
                  <div className="ps-summary">
                    <span>{c.order}</span>
                    <span className="ps-status">{c.pending}</span>
                  </div>
                )}
              </div>
            </li>
          ))}
        </ol>
      </section>
    </>
  );
}
