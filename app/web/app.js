"use strict";

const SESSION_KEY = "qa-session";
const MAX_POST_CHARACTERS = 280;

const byId = (id) => document.getElementById(id);

let session = JSON.parse(sessionStorage.getItem(SESSION_KEY));
let busy = false;

async function api(method, path, { token, body } = {}) {
  const headers = {};
  if (token) headers.Authorization = `Bearer ${token}`;
  if (body) headers["Content-Type"] = "application/json";
  let response;
  try {
    response = await fetch(path, { method, headers, body: body && JSON.stringify(body), cache: "no-store" });
  } catch {
    throw new Error("Could not reach the server");
  }
  if (!response.ok) {
    throw new Error((await response.text()).trim() || `Request failed (${response.status})`);
  }
  return response.json();
}

const roomPath = () => `/rooms/${encodeURIComponent(session.code)}`;

function showError(message) {
  byId("error").textContent = message;
  byId("error").hidden = !message;
}

// One request chain at a time, so a double tap cannot toggle a vote twice.
async function perform(action) {
  if (busy) return;
  busy = true;
  document.body.setAttribute("aria-busy", "true");
  showError("");
  try {
    await action();
  } catch (error) {
    showError(error.message);
  } finally {
    busy = false;
    document.body.removeAttribute("aria-busy");
  }
}

function saveSession() {
  sessionStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

function showScreen() {
  byId("home").hidden = Boolean(session);
  byId("room").hidden = !session;
  if (!session) return;
  const isHost = session.role === "host";
  byId("host-controls").hidden = !isHost;
  byId("post-form").hidden = isHost;
  byId("room-code").textContent = session.code;
  byId("alias").textContent = session.alias ?? "";
}

function clearRoom() {
  byId("room-state").textContent = "";
  byId("participant-count").textContent = "";
  byId("open-room").disabled = true;
  byId("close-room").disabled = true;
  byId("feed").replaceChildren();
  byId("empty-feed").hidden = true;
  byId("post-form").reset();
  byId("sort").value = "newest";
  showPostLength();
}

function showRoom(room) {
  byId("room-state").textContent = room.state;
  byId("participant-count").textContent = room.participantCount;
  byId("open-room").disabled = room.state !== "draft";
  byId("close-room").disabled = room.state !== "open";
}

function textElement(tag, className, text) {
  const element = document.createElement(tag);
  element.className = className;
  element.textContent = text;
  return element;
}

function voteButton(post) {
  const voted = session.votedPostIds.includes(post.id);
  const button = textElement("button", "", voted ? "Remove vote" : "Upvote");
  button.type = "button";
  button.setAttribute("aria-pressed", String(voted));
  button.addEventListener("click", () => perform(() => toggleVote(post.id)));
  return button;
}

function postItem(post) {
  const votes = document.createElement("div");
  votes.className = "row votes";
  votes.append(textElement("span", "", `${post.votes} ${post.votes === 1 ? "vote" : "votes"}`));
  if (session.role === "attendee") votes.append(voteButton(post));
  const item = document.createElement("li");
  item.append(textElement("p", "author", post.alias), textElement("p", "text", post.text), votes);
  return item;
}

function showFeed(posts) {
  byId("feed").replaceChildren(...posts.map(postItem));
  byId("empty-feed").hidden = posts.length > 0;
}

async function refresh() {
  showRoom(await api("GET", roomPath()));
  const sort = encodeURIComponent(byId("sort").value);
  const feed = await api("GET", `${roomPath()}/posts?sort=${sort}`, { token: session.token });
  showFeed(feed.posts);
}

async function enterRoom(next) {
  session = next;
  saveSession();
  clearRoom();
  showScreen();
  await refresh();
}

async function createRoom() {
  const room = await api("POST", "/rooms");
  await enterRoom({ role: "host", code: room.code, token: room.hostToken });
}

async function joinRoom() {
  const code = byId("join-code").value.toUpperCase();
  const joined = await api("POST", `/rooms/${encodeURIComponent(code)}/participants`);
  byId("join-form").reset();
  await enterRoom({ role: "attendee", code, token: joined.token, alias: joined.alias, votedPostIds: [] });
}

async function changeRoom(change) {
  await api("POST", `${roomPath()}/${change}`, { token: session.token });
  await refresh();
}

async function submitPost() {
  await api("POST", `${roomPath()}/posts`, { token: session.token, body: { text: byId("post-text").value } });
  byId("post-form").reset();
  showPostLength();
  await refresh();
}

// The API does not say whether the caller has voted, so the tab remembers it.
async function toggleVote(postId) {
  await api("POST", `${roomPath()}/posts/${postId}/vote`, { token: session.token });
  const voted = session.votedPostIds;
  session.votedPostIds = voted.includes(postId) ? voted.filter((id) => id !== postId) : [...voted, postId];
  saveSession();
  await refresh();
}

function showPostLength() {
  const length = [...byId("post-text").value].length;
  byId("post-length").textContent = `${length} / ${MAX_POST_CHARACTERS} characters`;
  byId("post-length").classList.toggle("over", length > MAX_POST_CHARACTERS);
}

function leaveRoom() {
  if (busy || !confirm("Leave this room? This tab cannot get back in as the same host or alias.")) return;
  session = null;
  sessionStorage.removeItem(SESSION_KEY);
  showError("");
  showScreen();
}

function onSubmit(formId, action) {
  byId(formId).addEventListener("submit", (event) => {
    event.preventDefault();
    perform(action);
  });
}

onSubmit("join-form", joinRoom);
onSubmit("post-form", submitPost);
byId("create-room").addEventListener("click", () => perform(createRoom));
byId("open-room").addEventListener("click", () => perform(() => changeRoom("open")));
byId("close-room").addEventListener("click", () => perform(() => changeRoom("close")));
byId("reload").addEventListener("click", () => perform(refresh));
byId("sort").addEventListener("change", () => perform(refresh));
byId("post-text").addEventListener("input", showPostLength);
byId("leave").addEventListener("click", leaveRoom);

clearRoom();
showScreen();
if (session) perform(refresh);
