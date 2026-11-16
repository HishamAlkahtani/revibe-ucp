// HTTP chunks can split an SSE frame or a UTF-8 character anywhere.
export async function readEvents(response, onEvent) {
  if (!response.body) throw new Error('Streaming is not supported by this client.');
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let finished = false;
  try {
    while (!finished) {
      const { value, done } = await reader.read();
      buffer += done ? decoder.decode() : decoder.decode(value, { stream: true });
      let boundary;
      while ((boundary = /\r?\n\r?\n/.exec(buffer))) {
        const frame = buffer.slice(0, boundary.index);
        buffer = buffer.slice(boundary.index + boundary[0].length);
        let event = 'message';
        const data = [];
        for (const line of frame.split(/\r?\n/)) {
          if (line.startsWith('event:')) event = line.slice(6).trim();
          if (line.startsWith('data:')) data.push(line.slice(5).replace(/^ /, ''));
        }
        if (!data.length) continue; // Ignore heartbeat comments.
        onEvent(event, JSON.parse(data.join('\n')));
        if (event === 'done' || event === 'error') { finished = true; break; }
      }
      if (done) break;
    }
    if (!finished) throw new Error('Connection ended before the reply finished. Refresh your cart before retrying an action.');
  } finally {
    try { await reader.cancel(); } finally { reader.releaseLock(); }
  }
}
