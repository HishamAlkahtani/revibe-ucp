import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readEvents } from '../app/stream.js';

test('SSE survives byte-by-byte UTF-8, CRLF, multiple events, and comments', async () => {
  const bytes = new TextEncoder().encode(': heartbeat\r\n\r\nevent: text_delta\r\ndata: {\r\ndata: "text":"مرحبا 🌍"}\r\n\r\nevent: done\ndata: {}\n\n');
  const response = new Response(new ReadableStream({
    start(controller) { for (const byte of bytes) controller.enqueue(new Uint8Array([byte])); controller.close(); }
  }));
  const events = [];
  await readEvents(response, (event, data) => events.push([event, data]));
  assert.deepEqual(events, [['text_delta', { text: 'مرحبا 🌍' }], ['done', {}]]);
});

test('EOF without a terminal event is an error; partial text is retained', async () => {
  const events = [];
  await assert.rejects(readEvents(new Response('event: text_delta\ndata: {"text":"partial"}\n\n'), (event, data) => events.push(data)), /before the reply finished/);
  assert.equal(events[0].text, 'partial');
});

test('error event terminates stream without waiting for connection closure', async () => {
  let canceled = false;
  const response = new Response(new ReadableStream({
    start(controller) { controller.enqueue(new TextEncoder().encode('event: error\ndata: {"message":"failed"}\n\n')); },
    cancel() { canceled = true; }
  }));
  let message;
  await readEvents(response, (event, data) => { message = data.message; });
  assert.equal(message, 'failed');
  assert.equal(canceled, true);
});
