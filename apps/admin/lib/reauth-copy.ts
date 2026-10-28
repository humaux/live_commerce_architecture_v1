// Purpose: every string of the recent re-authentication dialog (contracts/merchant-password-auth-v1.md §16, §11) in zh-TW, zh-CN, en.
// Depends on: @live-commerce/i18n (Locale) only; no React, no network.
// Used by: components/ReauthDialog.tsx; tests/admin/oc/reauth.spec.ts reads the rendered text, never this module.
// Invariants: wrong password, wrong code and refusals never say "signed out" (a 403/422 keeps the session, §16); the OIDC hint on
//   mail_unavailable is a separate string the dialog shows only when OIDC is configured AND the account has an OIDC identity (§11, PD9).
import type { Locale } from "@live-commerce/i18n";

const en = {
  title: "Confirm it's you",
  introPassword: "This action needs a recent sign-in. Enter your password and we will email you a code.",
  introOidc: "This action needs a recent sign-in. Sign in again with your identity provider; you stay on this page.",
  password: "Password",
  sendCode: "Send code",
  code: "6-digit code",
  codeSent: "We sent a code to your account email. It expires in 10 minutes. Check your spam folder if it is not there.",
  verify: "Confirm",
  startOver: "Send a new code",
  oidc: "Sign in with identity provider",
  usePassword: "Use password instead",
  useOidc: "Use identity provider instead",
  cancel: "Cancel",
  working: "Working…",
  waitingOidc: "Finish signing in in the window that opened. This dialog continues when you are done.",
  errors: {
    reauth_failed: "That did not confirm your identity. Check the password (or the account you signed in with) and try again.",
    invalid_code: "That code is not correct or has expired. Check it or send a new code.",
    throttled: "Too many attempts. Wait a few minutes and try again.",
    mail_unavailable: "We cannot send email right now, so this action is blocked for the moment. Try again later.",
    busy: "Service busy. Try again shortly.",
    unauthorized: "Your session has ended. Sign in again.",
    popup_blocked: "The sign-in window was blocked. Allow pop-ups for this site and try again.",
    retry_later: "Temporarily unavailable. Try again.",
  } as Record<string, string>,
  mailUnavailableOidc: "You can confirm with your identity provider instead.",
};

const zhCN: typeof en = {
  title: "确认是你本人",
  introPassword: "此操作需要最近登录过。请输入密码，我们会向你的邮箱发送验证码。",
  introOidc: "此操作需要最近登录过。请通过身份提供方重新登录；你会留在当前页面。",
  password: "密码",
  sendCode: "发送验证码",
  code: "6 位验证码",
  codeSent: "验证码已发送到你的账号邮箱，10 分钟内有效。如未收到，请查看垃圾邮件。",
  verify: "确认",
  startOver: "重新发送验证码",
  oidc: "通过身份提供方登录",
  usePassword: "改用密码",
  useOidc: "改用身份提供方",
  cancel: "取消",
  working: "处理中…",
  waitingOidc: "请在新打开的窗口完成登录，完成后此对话框会继续。",
  errors: {
    reauth_failed: "未能确认身份。请检查密码（或登录所用的账号）后重试。",
    invalid_code: "验证码不正确或已过期，请检查或重新发送。",
    throttled: "尝试次数过多，请等几分钟后再试。",
    mail_unavailable: "暂时无法发送邮件，此操作暂时无法进行，请稍后再试。",
    busy: "服务繁忙，请稍后再试。",
    unauthorized: "登录已失效，请重新登录。",
    popup_blocked: "登录窗口被拦截。请允许此网站的弹出窗口后重试。",
    retry_later: "暂时无法使用，请重试。",
  },
  mailUnavailableOidc: "你也可以改用身份提供方确认。",
};

const zhTW: typeof en = {
  title: "確認是你本人",
  introPassword: "此操作需要最近登入過。請輸入密碼，我們會寄送驗證碼到你的信箱。",
  introOidc: "此操作需要最近登入過。請透過身分提供者重新登入；你會留在目前頁面。",
  password: "密碼",
  sendCode: "寄送驗證碼",
  code: "6 位數驗證碼",
  codeSent: "驗證碼已寄到你的帳號信箱，10 分鐘內有效。若沒收到，請查看垃圾郵件。",
  verify: "確認",
  startOver: "重新寄送驗證碼",
  oidc: "透過身分提供者登入",
  usePassword: "改用密碼",
  useOidc: "改用身分提供者",
  cancel: "取消",
  working: "處理中…",
  waitingOidc: "請在新開啟的視窗完成登入，完成後此對話框會繼續。",
  errors: {
    reauth_failed: "未能確認身分。請檢查密碼（或登入所用的帳號）後重試。",
    invalid_code: "驗證碼不正確或已過期，請檢查或重新寄送。",
    throttled: "嘗試次數過多，請等幾分鐘後再試。",
    mail_unavailable: "暫時無法寄送郵件，此操作暫時無法進行，請稍後再試。",
    busy: "服務忙碌中，請稍後再試。",
    unauthorized: "登入已失效，請重新登入。",
    popup_blocked: "登入視窗被封鎖。請允許此網站的彈出式視窗後重試。",
    retry_later: "暫時無法使用，請重試。",
  },
  mailUnavailableOidc: "你也可以改用身分提供者確認。",
};

export const reauthCopy: Record<Locale, typeof en> = { en, "zh-CN": zhCN, "zh-TW": zhTW };
export const reauthError = (c: typeof en, code: string): string => c.errors[code] ?? c.errors.retry_later;
