import Chat from './chat';

// The static shell is a Server Component. Only the chat is interactive.
export default function Page() {
  return <>
    <header><a href="/">revibe<span> / shopping assistant</span></a><small>Saudi Arabia · SAR</small></header>
    <main>
      <section className="intro"><p className="eyebrow">RENEWED TECH. A NEW CONVERSATION.</p><h1>Find your next device.</h1><p>Tell me what you’re looking for. I’ll check Revibe’s live catalog and help you get to checkout.</p></section>
      <Chat />
      <footer>Powered by Revibe UCP · Payment is completed on Revibe.</footer>
    </main>
  </>;
}
