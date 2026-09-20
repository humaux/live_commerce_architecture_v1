const cells: Record<string, number> = {
  "HA-001-BE": 0,
  "AC-002-BK": 1,
  "CL-003-SET": 2,
  "ET-004-S": 3,
  "ET-004-M": 4,
  "ET-004-L": 5,
  "CB-005-TC": 6,
  "AC-006-GY": 7,
  "CL-007-BR": 8,
};
export function ProductPhoto({
  code,
  name,
  demo,
  large = false,
}: {
  code: string;
  name: string;
  demo: boolean;
  large?: boolean;
}) {
  const cell = cells[code];
  return (
    <div
      className={`product-photo${large ? " large" : ""}`}
      role="img"
      aria-label={name}
      style={
        demo && cell !== undefined
          ? {
              backgroundImage:
                "url(/demo-assets/hearing-aid-accessory-atlas.png)",
              backgroundSize: "300% 300%",
              backgroundPosition: `${(cell % 3) * 50}% ${Math.floor(cell / 3) * 50}%`,
            }
          : undefined
      }
    >
      {(!demo || cell === undefined) && (
        <span aria-hidden="true">{name.slice(0, 1)}</span>
      )}
    </div>
  );
}
