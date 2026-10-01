package identity

// Mail content for staff invitations (contracts/storefront-v2.md §D). One message, in the INVITER's admin locale, with
// one link to /{locale}/invite/{token} on the configured public origin. Unlike the login code mails this one
// necessarily carries a secret in a URL (the one-time token); that is acceptable because the token is single use,
// expires in 72 h, is useless without a signed-in account whose verified email equals the invited one, and the accept
// page sets Referrer-Policy no-referrer. No images, no tracking pixels.

import (
	"html"
	"strings"

	"livecommerce/internal/mail"
)

type staffCopyText struct{ Subject, Intro, Role, Action, Valid, Ignore string }

// staffRoleNames are the five role labels per locale (admin copy uses the same words in lib/team-copy.ts).
var staffRoleNames = map[string]map[string]string{
	"en":    {"owner": "Owner", "admin": "Admin", "live_operator": "Live operator", "fulfilment": "Fulfilment", "viewer": "Viewer"},
	"zh-CN": {"owner": "所有者", "admin": "管理员", "live_operator": "直播运营", "fulfilment": "履约", "viewer": "只读成员"},
	"zh-TW": {"owner": "擁有者", "admin": "管理員", "live_operator": "直播營運", "fulfilment": "履約", "viewer": "唯讀成員"},
}

var staffCopy = map[string]staffCopyText{
	"en":    {"You are invited to join a store team", "You have been invited to join a store team on this platform.", "Your role:", "Open this link, sign in or sign up with this email address, then accept the invitation:", "The link works once and is valid for 72 hours.", "If you did not expect this, ignore this email; nothing happens unless you accept."},
	"zh-CN": {"邀请您加入店铺团队", "您被邀请加入本平台上的一个店铺团队。", "您的角色：", "请打开以下链接，使用此邮箱地址登录或注册，然后接受邀请：", "链接仅可使用一次，72 小时内有效。", "如果您并不期待这封邮件，请忽略；不接受邀请就不会有任何变化。"},
	"zh-TW": {"邀請您加入店鋪團隊", "您被邀請加入本平台上的一個店鋪團隊。", "您的角色：", "請開啟以下連結，使用此電子郵件地址登入或註冊，然後接受邀請：", "連結僅可使用一次，72 小時內有效。", "如果您並不期待這封郵件，請忽略；不接受邀請就不會有任何變化。"},
}

// staffInviteMail builds the invitation mail. link already contains the token; role and locale are validated by the caller.
func staffInviteMail(to, locale, role, link string) mail.Message {
	c, roles := staffCopy[locale], staffRoleNames[locale]
	text := strings.Join([]string{c.Intro, c.Role + " " + roles[role], "", c.Action, link, "", c.Valid, c.Ignore, ""}, "\n")
	page := "<p>" + html.EscapeString(c.Intro) + "</p><p>" + html.EscapeString(c.Role) + " <strong>" + html.EscapeString(roles[role]) + "</strong></p>" +
		"<p>" + html.EscapeString(c.Action) + "</p><p><a href=\"" + html.EscapeString(link) + "\">" + html.EscapeString(link) + "</a></p>" +
		"<p>" + html.EscapeString(c.Valid) + "</p><p>" + html.EscapeString(c.Ignore) + "</p>"
	return mail.Message{To: to, Subject: c.Subject, Text: text, HTML: page}
}
