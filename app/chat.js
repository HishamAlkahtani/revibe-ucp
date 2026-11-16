'use client';

import { useEffect, useRef, useState } from 'react';
import { readEvents } from './stream';

const money = amount => new Intl.NumberFormat('en-SA', { style: 'currency', currency: 'SAR' }).format(amount / 100);
const toolLabels = { get_cart: 'Checking your cart...', search_products: 'Searching Revibe…', add_to_cart: 'Updating your cart…', prepare_checkout: 'Preparing checkout…' };

function ProductCard({ product }) {
  const image = product.variants.find(variant => variant.image)?.image;
  return <article>
    {image && <img src={image} alt={product.title} loading="lazy" />}
    <h2>{product.title}</h2>
    <ul className="variants">
      {product.variants.map(variant => <li key={variant.id}>
        {variant.title} &middot; {money(variant.price.amount)}
      </li>)}
    </ul>
  </article>;
}

export default function Chat() {
  const [messages, setMessages] = useState([{ text: 'Hi! Try “Find an iPhone 13 under 1,500 SAR”.', type: 'assistant' }]);
  const [message, setMessage] = useState('');
  const [products, setProducts] = useState([]);
  const [cart, setCart] = useState(null);
  const [checkoutUrl, setCheckoutUrl] = useState('');
  const [status, setStatus] = useState('');
  const [busy, setBusy] = useState(false);
  const inFlight = useRef(false);
  const input = useRef(null);
  const controller = useRef(null);
  useEffect(() => () => controller.current?.abort(), []);

  function updateState(data) {
    if (data.products) setProducts(data.products);
    if ('cart' in data) setCart(data.cart);
    if ('checkoutUrl' in data) setCheckoutUrl(data.checkoutUrl || '');
  }

  function updateReply(text, type = 'assistant', append = false) {
    setMessages(previous => previous.map((m, i) => i === previous.length - 1
      ? { text: append ? m.text + text : text, type } : m));
  }

  function receive(event, data) {
    if (event === 'text_delta') updateReply(data.text, 'assistant', true);
    if (event === 'tool_started') setStatus(toolLabels[data.name] || 'Working…');
    if (event === 'tool_finished') setStatus(data.ok ? 'Writing reply…' : 'Checking another option…');
    if (event === 'products') setProducts(data);
    if (event === 'cart') { setCart(data); setCheckoutUrl(''); }
    if (event === 'checkout_ready') setCheckoutUrl(data.url || '');
    if (event === 'state') updateState(data);
    if (event === 'done') setStatus('');
    if (event === 'error') { updateReply(`\n${data.message}`, 'error', true); setStatus(''); }
  }

  async function send(text) {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setMessage('');
    setCheckoutUrl('');
    setStatus('Working…');
    setMessages(previous => [...previous, { text, type: 'user' }, { text: '', type: 'assistant' }]);
    controller.current = new AbortController();
    try {
      const response = await fetch('/api/chat', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: text }),
        signal: controller.current.signal
      });
      if (response.ok && response.headers.get('content-type')?.includes('text/event-stream')) {
        await readEvents(response, receive);
      } else {
        const data = await response.json();
        updateState(data);
        updateReply(data.error || data.reply || 'Unable to complete the request.', data.error || !response.ok ? 'error' : 'assistant');
        setStatus('');
      }
    } catch (error) {
      if (error.name === 'AbortError') {
        updateReply('\nStopped.', 'assistant', true);
        setStatus('Stopped. A cart action may have completed; ask me to check your cart before retrying.');
      } else {
        updateReply(`\n${error.message || 'Connection failed.'}`, 'error', true);
        setStatus('Ask me to check your cart before retrying a shopping action.');
      }
    } finally {
      inFlight.current = false;
      setBusy(false);
      controller.current = null;
      input.current?.focus();
    }
  }

  const total = cart?.totals?.find(t => t.type === 'total');
  return <>
      <section id="messages" aria-label="Conversation" aria-live="polite">
        {messages.map((m, i) => <p key={i} className={m.type} dir="auto">{m.text}</p>)}
      </section>
      <p id="status" role="status">{status}</p>
      <section id="products" aria-label="Matching devices">
        {products.map(p => <ProductCard key={`${p.title}:${p.variants.map(v => v.id).join(',')}`} product={p} />)}
      </section>
      {cart && <aside id="cart">
        <h2>Your cart</h2>
        {cart.line_items?.map(line => <p key={line.item.id}>{line.quantity} × {line.item.title}</p>)}
        {total && <p>Estimated total: {money(total.amount)}</p>}
        <p className="muted">Estimated totals. Confirm shipping and payment on Revibe.</p>
        {checkoutUrl && <a id="checkout" href={checkoutUrl} target="_blank" rel="noopener noreferrer">Continue to Revibe ↗</a>}
      </aside>}
      <form onSubmit={event => { event.preventDefault(); if (message.trim()) send(message.trim()); }}>
        <label className="sr-only" htmlFor="message">Message</label>
        <input ref={input} id="message" value={message} onChange={event => setMessage(event.target.value)} placeholder="Ask about a device…" autoComplete="off" maxLength={2000} required />
        <button disabled={busy} type="submit">Send ↑</button>
        {busy && controller.current && <button type="button" onClick={() => controller.current?.abort()}>Stop</button>}
      </form>
  </>;
}
