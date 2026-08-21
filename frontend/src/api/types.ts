// The API contract as the frontend sees it, mirroring backend/internal/dtos.
// Components import from here and never read raw response fields.
//
// No response envelope: endpoints return the payload directly and signal
// success with the status code. Errors are normalised into ApiError in
// client.ts.

// Timestamps arrive as ISO 8601 strings; parse where they're displayed.
export type Timestamp = string;

export interface Paginated<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

// --- Listings ---

export interface ListingImage {
  id: string;
  url: string; // relative: "/uploads/abc.jpg"
  position: number;
}

export interface Listing {
  id: string;
  seller_id: string;
  title: string;
  description: string;
  category: string;
  price: number;
  quantity: number;
  unit: string;
  created_at: Timestamp;
  updated_at: Timestamp;
  images: ListingImage[]; // always an array, never null
}

export type OrderStatus = "pending" | "confirmed" | "completed" | "cancelled";

export interface Order {
  id: string;
  listing_id: string;
  listing_title: string; // snapshotted at order time
  buyer_id: string;
  seller_id: string;
  quantity: number;
  // The backend sends these as strings ("18.00") while listing price is a
  // number. Normalised to number in orders.ts.
  unit_price: number;
  total_price: number;
  status: OrderStatus;
  // Null until that side confirms; both set means completed.
  seller_handed_over_at: Timestamp | null;
  buyer_received_at: Timestamp | null;
  created_at: Timestamp;
  updated_at: Timestamp;
}

// --- Chat ---

export interface Presence {
  is_online: boolean;
  last_seen_at?: Timestamp; // absent when hidden AND when never seen
}

export interface ChatUser {
  id: string;
  username: string;
  presence: Presence;
}

export type ConversationStatus = "pending" | "accepted" | "declined";
export type ConversationRole = "buyer" | "seller";

export interface Conversation {
  id: string;
  listing_id: string | null; // null once the listing is deleted
  listing_title: string;
  status: ConversationStatus;
  role: ConversationRole;
  other_user: ChatUser;
  created_at: Timestamp;
  updated_at: Timestamp;
}

export interface MessagePreview {
  body: string;
  created_at: Timestamp;
}

export interface ConversationListItem extends Omit<Conversation, "created_at"> {
  last_message: MessagePreview | null;
  unread_count: number;
}

export interface Message {
  id: string;
  conversation_id: string;
  sender_id: string;
  body: string;
  read_at?: Timestamp; // absent while unread
  created_at: Timestamp;
}

// --- Me ---

export interface UserSettings {
  show_online_status: boolean;
}

export interface UnreadCount {
  unread_count: number;
}

// Auth Foundation additions
export interface User {
  id: string;
  username: string;
  email: string;
  role: string;
}

export interface SignupInput {
  username: string;
  email: string;
  password: string;
}

// Login is by email only - a username is a display name, not a credential.
export interface LoginInput {
  email: string;
  password: string;
}

// Returned by signup, login and refresh alike, so one code path can start a
// session from any of them. The refresh token itself is never here: it travels
// as an HttpOnly cookie.
export interface AuthResponse {
  access_token: string;
  user: User;
}

// GET /me/profile. Unset fields arrive as null rather than absent, so forms
// can render an empty input without deciding whether the key exists.
export interface OwnProfile {
  id: string;
  username: string;
  email: string;
  firstname: string | null;
  lastname: string | null;
  bio: string | null;
  phone_number: string | null;
  date_of_birth: string | null;
  location: string | null;
}

// PATCH /me/profile body. Each field has three states: key absent keeps the
// current value, "" or null clears it, a value replaces it (trimmed). Username
// and email are identity, not profile - they are not updatable here.
export type ProfileUpdateInput = Partial<{
  firstname: string | null;
  lastname: string | null;
  bio: string | null;
  phone_number: string | null;
  date_of_birth: string | null;
  location: string | null;
}>;

// TODO: add an ApiError interface? e.g.
// export interface ApiError {
//   error: string;
//   details?: string;
// }

// NOTE: there is no response envelope. endpoints return the payload directly
// and signal success/failure through the HTTP status code. errors come back as
// { error, details } - the interceptor is where this will be normalised
