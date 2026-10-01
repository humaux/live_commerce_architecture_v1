// Authored 24px stroke icons for the storefront shell (one weight, one grid). Server and client components alike; no
// BFF/Go. Decorative by default (aria-hidden): the button or link around an icon carries the accessible name.
import type { ReactNode } from "react";

function Icon({ children, size = 22 }: { children: ReactNode; size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {children}
    </svg>
  );
}

export const MenuIcon = () => (
  <Icon>
    <path d="M4 7h16M4 12h16M4 17h16" />
  </Icon>
);
export const CloseIcon = () => (
  <Icon>
    <path d="M6 6l12 12M18 6L6 18" />
  </Icon>
);
export const SearchIcon = () => (
  <Icon>
    <circle cx="11" cy="11" r="6.5" />
    <path d="M16 16l4.5 4.5" />
  </Icon>
);
export const BagIcon = () => (
  <Icon>
    <path d="M5.5 8h13l-1 11.5a1 1 0 0 1-1 .9H7.5a1 1 0 0 1-1-.9L5.5 8z" />
    <path d="M9 8V7a3 3 0 0 1 6 0v1" />
  </Icon>
);
export const PlusIcon = () => (
  <Icon size={18}>
    <path d="M12 5v14M5 12h14" />
  </Icon>
);
export const MinusIcon = () => (
  <Icon size={18}>
    <path d="M5 12h14" />
  </Icon>
);
export const ChevronRightIcon = () => (
  <Icon size={18}>
    <path d="M9 6l6 6-6 6" />
  </Icon>
);
export const ArrowRightIcon = () => (
  <Icon size={18}>
    <path d="M5 12h14M13 6l6 6-6 6" />
  </Icon>
);
export const CheckIcon = () => (
  <Icon size={18}>
    <path d="M5 12.5l4.5 4.5L19 7.5" />
  </Icon>
);
export const TrashIcon = () => (
  <Icon size={18}>
    <path d="M4.5 7h15M10 7V5h4v2M7 7l.8 11.2a1 1 0 0 0 1 .8h6.4a1 1 0 0 0 1-.8L17 7" />
  </Icon>
);
export const FilterIcon = () => (
  <Icon size={18}>
    <path d="M4 7h10M18 7h2M4 17h2M10 17h10" />
    <circle cx="16" cy="7" r="2" />
    <circle cx="8" cy="17" r="2" />
  </Icon>
);
export const MailIcon = () => (
  <Icon size={18}>
    <rect x="3.5" y="5.5" width="17" height="13" rx="2" />
    <path d="M4 7l8 6 8-6" />
  </Icon>
);
export const PhoneIcon = () => (
  <Icon size={18}>
    <path d="M6.5 4h3l1.5 4-2 1.3a10 10 0 0 0 5.7 5.7L16 13l4 1.5v3a2 2 0 0 1-2 2A13.5 13.5 0 0 1 4.5 6a2 2 0 0 1 2-2z" />
  </Icon>
);
export const PinIcon = () => (
  <Icon size={18}>
    <path d="M12 21s6.5-5.6 6.5-11a6.5 6.5 0 0 0-13 0C5.5 15.4 12 21 12 21z" />
    <circle cx="12" cy="10" r="2.3" />
  </Icon>
);
export const ChatIcon = () => (
  <Icon size={18}>
    <path d="M20 11.5c0 3.6-3.6 6.5-8 6.5-.9 0-1.7-.1-2.5-.3L5 19.5l1.2-3.4C4.8 15 4 13.3 4 11.5 4 7.9 7.6 5 12 5s8 2.9 8 6.5z" />
  </Icon>
);
export const FacebookIcon = () => (
  <Icon size={18}>
    <path d="M14 8.5h2.5V5H14a3.5 3.5 0 0 0-3.5 3.5V11H8v3.5h2.5V20H14v-5.5h2.3l.4-3.5H14V8.5z" />
  </Icon>
);
export const InstagramIcon = () => (
  <Icon size={18}>
    <rect x="4" y="4" width="16" height="16" rx="4.5" />
    <circle cx="12" cy="12" r="3.6" />
    <circle cx="16.8" cy="7.2" r="0.6" fill="currentColor" />
  </Icon>
);
export const GlobeIcon = () => (
  <Icon size={18}>
    <circle cx="12" cy="12" r="8.5" />
    <path d="M3.5 12h17M12 3.5c2.5 2.3 3.5 5.2 3.5 8.5s-1 6.2-3.5 8.5c-2.5-2.3-3.5-5.2-3.5-8.5s1-6.2 3.5-8.5z" />
  </Icon>
);
