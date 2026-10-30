# Project chat reads in the unified account notification module

## User flows

- Desktop and tablet use a master-detail project chat workspace: the chat list remains on the left and the selected conversation is on the right.
- Mobile opens the chat list first. Selecting a chat opens its conversation; the conversation header contains a visible back action to the project chat list.
- Settings and delete actions are fixed in each manageable chat row. The conversation pane contains only the message history and composer; mobile keeps only a compact back action.
- A chat manager (chat creator, project administrator, super administrator or technical administrator) can rename a chat, add an active project participant, remove a removable participant and soft-delete the chat. Chat membership never grants project access: a registered user must first be added to the project, which the empty state explains and links to.
- An account receives an unread count per project chat and the same global notification feed used by tasks, meetings, finances, materials, documents and project membership events across every accessible project, including projects that are not currently open.

## Data and authorization

- `chat_memberships.last_read_at` is the per-user read marker. It is advanced only to a message that belongs to the same accessible chat.
- `chats.archived_at` soft-deletes a chat. Messages and memberships are retained for audit and are excluded from normal reads.
- `account_notifications` is the application-wide inbox owned by `recipient_user_id`; it is not chat-owned. Reads and read mutations are always scoped to the authenticated account.
- Notification recipients reuse the same project-role and privilege predicates as Telegram delivery. Chat notifications are restricted to active members of the specific chat.
- Telegram outbox delivery remains independent; the in-app feed does not require a Telegram binding.

## Delivery and polling

- The project chat list and the selected conversation refresh every 10 seconds and when the window regains focus.
- The global notification center refreshes every 20 seconds and when the window regains focus.
- Updates use one atomic screen-reader status message. Badges themselves are not competing live regions.
- Polling is intentionally bounded for R1. A later realtime transport can replace it without changing the read-marker or notification contracts.

## Retention

Chat history follows the project retention policy. In-app notification retention is currently application-managed and must be assigned an operational purge window before production volume requires partitioning or archival.
