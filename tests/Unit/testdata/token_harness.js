'use strict';

// Runs assets/turnstile.js against a stand-in for the page and for
// Cloudflare's script, and prints one JSON line per scenario:
// {"name": "...", "error": ""} -- error is empty when the scenario held.
//
// No network and no browser: the page is a handful of objects, Cloudflare's
// turnstile object records what it was asked to do, and a scenario plays the
// part of Cloudflare by calling the callbacks it was given.
//
// Usage: node token_harness.js <path to turnstile.js>

const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

const source = fs.readFileSync(process.argv[2], 'utf8');
const later = () => new Promise((resolve) => setImmediate(resolve));
const settle = async () => { for (let i = 0; i < 4; i++) await later(); };

function element(tag) {
  const node = {
    tagName: String(tag).toUpperCase(), attributes: {}, children: [], parentNode: null, listeners: {},
    setAttribute(name, value) { node.attributes[name] = String(value); },
    getAttribute(name) { return name in node.attributes ? node.attributes[name] : null; },
    appendChild(child) { child.parentNode = node; node.children.push(child); if (node.onAppend) node.onAppend(child); return child; },
    removeChild(child) { node.children = node.children.filter((c) => c !== child); child.parentNode = null; return child; },
    addEventListener(name, fn) { (node.listeners[name] = node.listeners[name] || []).push(fn); },
    removeEventListener(name, fn) { node.listeners[name] = (node.listeners[name] || []).filter((f) => f !== fn); },
    fire(name) { for (const fn of (node.listeners[name] || []).slice()) fn({ type: name }); },
  };
  return node;
}

// page builds the stand-in and runs the script in it. cloudflare is "loads"
// (the script arrives and defines window.turnstile), "blocked" (the policy
// refuses it) or "empty" (it arrives and defines nothing).
function page({ cloudflare = 'loads', ui = false, marked = false } = {}) {
  const env = { widgets: [], scripts: [], errors: [], definitions: {}, document: null, window: null };

  const turnstile = {
    render(container, options) {
      const widget = { id: 'w' + (env.widgets.length + 1), container, options, removed: false, resets: 0 };
      env.widgets.push(widget);
      return widget.id;
    },
    reset(id) { env.widgets.find((w) => w.id === id).resets++; },
    remove(id) { env.widgets.find((w) => w.id === id).removed = true; },
  };

  const html = element('html');
  const head = element('head');
  const body = element('body');
  html.children.push(head, body);
  head.onAppend = (child) => {
    if (child.tagName !== 'SCRIPT') return;
    env.scripts.push(child);
    setImmediate(() => {
      if (cloudflare === 'blocked') { child.fire('error'); return; }
      if (cloudflare === 'loads') env.window.turnstile = turnstile;
      child.fire('load');
    });
  };

  const document = {
    head, body, documentElement: { contains: () => true },
    createElement: element,
    querySelector: (selector) => (marked && selector.includes('data-kyse-behavior') ? element('div') : null),
  };
  const window = {
    document, console: { error: (message) => env.errors.push(String(message)), warn() {} },
    setTimeout, clearTimeout,
  };
  window.window = window;
  if (ui) window.arandu = { ui: { define: (name, hooks) => { env.definitions[name] = hooks; } } };
  if (cloudflare === 'preloaded') window.turnstile = turnstile;

  env.document = document;
  env.window = window;
  env.run = () => vm.runInContext(source, env.context);
  env.context = vm.createContext(window);
  env.run();
  env.api = () => window.arandu.turnstile;
  return env;
}

const scenarios = {
  async 'a token is asked for without a form, a component or arandu.ui'() {
    const env = page();
    assert.equal(env.window.arandu.ui, undefined);
    const asked = env.api().token({ sitekey: 'site', action: 'card-tokenization', cData: 'abc' });
    await settle();
    assert.equal(env.widgets.length, 1);
    const { options, container } = env.widgets[0];
    assert.equal(options.sitekey, 'site');
    assert.equal(options.action, 'card-tokenization');
    assert.equal(options.cData, 'abc');
    assert.equal(options.appearance, 'interaction-only');
    assert.equal(options['response-field'], false);
    assert.ok(container.getAttribute('data-turnstile-token') !== null);
    assert.equal(container.parentNode, env.document.body);
    options.callback('the-token');
    assert.equal(await asked, 'the-token');
  },

  async 'the widget and the element added for it are removed once the token is read'() {
    const env = page();
    const asked = env.api().token({ sitekey: 'site' });
    await settle();
    env.widgets[0].options.callback('t');
    await asked;
    assert.equal(env.widgets[0].removed, true);
    assert.equal(env.document.body.children.length, 0);
  },

  async 'an option that was not written is not passed'() {
    const env = page();
    env.api().token({ sitekey: 'site' });
    await settle();
    for (const name of ['action', 'cData', 'theme', 'size', 'language']) {
      assert.equal(name in env.widgets[0].options, false, name);
    }
  },

  async 'the options of the caller reach Cloudflare'() {
    const env = page();
    env.api().token({ sitekey: 'site', theme: 'dark', size: 'compact', language: 'pt-BR', appearance: 'always' });
    await settle();
    const { options } = env.widgets[0];
    assert.equal(options.theme, 'dark');
    assert.equal(options.size, 'compact');
    assert.equal(options.language, 'pt-BR');
    assert.equal(options.appearance, 'always');
  },

  async 'Cloudflare is not asked to retry or to refresh on its own'() {
    const env = page();
    env.api().token({ sitekey: 'site' });
    await settle();
    assert.equal(env.widgets[0].options.retry, 'never');
    assert.equal(env.widgets[0].options['refresh-expired'], 'never');
  },

  async 'the script is added once for several tokens, and only to the head'() {
    const env = page();
    const a = env.api().token({ sitekey: 'site' });
    const b = env.api().token({ sitekey: 'site' });
    await settle();
    assert.equal(env.scripts.length, 1);
    assert.equal(env.scripts[0].src, 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit');
    assert.equal(env.widgets.length, 2);
    env.widgets[0].options.callback('first');
    env.widgets[1].options.callback('second');
    assert.deepEqual([await a, await b], ['first', 'second']);
  },

  async 'a second load of the script changes nothing'() {
    const env = page();
    const api = env.api();
    env.run();
    assert.equal(env.api(), api);
  },

  async 'a script that Cloudflare was not allowed to load rejects as unavailable'() {
    const env = page({ cloudflare: 'blocked' });
    await assert.rejects(env.api().token({ sitekey: 'site' }), (error) => error.code === 'challenge_unavailable');
    assert.equal(env.errors.length, 1);
    assert.match(env.errors[0], /turnstile\.Headers/);
    assert.equal(env.document.body.children.length, 0);
  },

  async 'a script that defines no turnstile object rejects as unavailable'() {
    const env = page({ cloudflare: 'empty' });
    await assert.rejects(env.api().token({ sitekey: 'site' }), (error) => error.code === 'challenge_unavailable');
  },

  async 'the load is tried again after it failed'() {
    const env = page({ cloudflare: 'blocked' });
    await assert.rejects(env.api().token({ sitekey: 'site' }));
    await assert.rejects(env.api().token({ sitekey: 'site' }));
    assert.equal(env.scripts.length, 2);
  },

  async 'an error of the challenge rejects as failed with Cloudflare\'s code'() {
    const env = page();
    const asked = env.api().token({ sitekey: 'site' });
    await settle();
    env.widgets[0].options['error-callback']('110200');
    await assert.rejects(asked, (error) => error.code === 'challenge_failed' && error.reason === '110200');
    assert.equal(env.widgets[0].removed, true);
    assert.equal(env.document.body.children.length, 0);
  },

  async 'a challenge a person left unsolved rejects as failed'() {
    const env = page();
    const asked = env.api().token({ sitekey: 'site' });
    await settle();
    env.widgets[0].options['timeout-callback']();
    await assert.rejects(asked, (error) => error.code === 'challenge_failed' && error.reason === 'timeout');
  },

  async 'a browser Turnstile does not support rejects as failed'() {
    const env = page();
    const asked = env.api().token({ sitekey: 'site' });
    await settle();
    env.widgets[0].options['unsupported-callback']();
    await assert.rejects(asked, (error) => error.code === 'challenge_failed' && error.reason === 'unsupported');
  },

  async 'a challenge that never answers rejects when the wait is over'() {
    const env = page();
    const asked = env.api().token({ sitekey: 'site', timeout: 20 });
    await assert.rejects(asked, (error) => error.code === 'challenge_failed' && error.reason === 'timeout');
    assert.equal(env.widgets[0].removed, true);
  },

  async 'a wait of zero is no wait at all'() {
    const env = page();
    const asked = env.api().token({ sitekey: 'site', timeout: 0 });
    await new Promise((resolve) => setTimeout(resolve, 60));
    env.widgets[0].options.callback('late');
    assert.equal(await asked, 'late');
  },

  async 'a missing site key rejects instead of throwing'() {
    const env = page();
    await assert.rejects(env.api().token({}), (error) => error.code === 'challenge_unconfigured');
    await assert.rejects(env.api().token(), (error) => error.code === 'challenge_unconfigured');
    assert.equal(env.scripts.length, 0);
  },

  async 'prepare with no site key throws, because nothing was started'() {
    const env = page();
    assert.throws(() => env.api().prepare({}), (error) => error.code === 'challenge_unconfigured');
  },

  async 'a token solved before it was asked for is handed over at once'() {
    const env = page();
    const handle = env.api().prepare({ sitekey: 'site', action: 'pay' });
    await settle();
    assert.equal(env.widgets.length, 1);
    env.widgets[0].options.callback('held');
    assert.equal(env.widgets[0].removed, false);
    assert.equal(await handle.token(), 'held');
    assert.equal(env.widgets[0].removed, true);
  },

  async 'a token held until it expires is solved again, and the new one is handed over'() {
    const env = page();
    const handle = env.api().prepare({ sitekey: 'site' });
    await settle();
    env.widgets[0].options.callback('old');
    env.widgets[0].options['expired-callback']();
    assert.equal(env.widgets[0].resets, 1);
    assert.equal(env.widgets.length, 1);
    const asked = handle.token();
    env.widgets[0].options.callback('new');
    assert.equal(await asked, 'new');
  },

  async 'a token that expired more than once is renewed each time'() {
    const env = page();
    const handle = env.api().prepare({ sitekey: 'site' });
    await settle();
    for (const token of ['one', 'two', 'three']) {
      env.widgets[0].options.callback(token);
      env.widgets[0].options['expired-callback']();
    }
    assert.equal(env.widgets[0].resets, 3);
    env.widgets[0].options.callback('four');
    assert.equal(await handle.token(), 'four');
  },

  async 'a handle gives one token'() {
    const env = page();
    const handle = env.api().prepare({ sitekey: 'site' });
    await settle();
    env.widgets[0].options.callback('only');
    assert.equal(await handle.token(), 'only');
    await assert.rejects(handle.token(), (error) => error.code === 'challenge_spent');
  },

  async 'two asks at once on one handle give one token'() {
    const env = page();
    const handle = env.api().prepare({ sitekey: 'site' });
    const first = handle.token();
    await assert.rejects(handle.token(), (error) => error.code === 'challenge_spent');
    await settle();
    env.widgets[0].options.callback('only');
    assert.equal(await first, 'only');
  },

  async 'cancel rejects the ask that is waiting and removes the widget'() {
    const env = page();
    const handle = env.api().prepare({ sitekey: 'site' });
    const asked = handle.token();
    await settle();
    handle.cancel();
    await assert.rejects(asked, (error) => error.code === 'challenge_cancelled');
    assert.equal(env.widgets[0].removed, true);
    assert.equal(env.document.body.children.length, 0);
  },

  async 'a cancelled handle gives nothing'() {
    const env = page();
    const handle = env.api().prepare({ sitekey: 'site' });
    await settle();
    handle.cancel();
    await assert.rejects(handle.token(), (error) => error.code === 'challenge_cancelled');
  },

  async 'cancelling before the script arrived draws nothing'() {
    const env = page();
    const handle = env.api().prepare({ sitekey: 'site' });
    handle.cancel();
    await settle();
    assert.equal(env.widgets.length, 0);
  },

  async 'a challenge that failed before it was asked for is solved again when asked'() {
    const env = page();
    const handle = env.api().prepare({ sitekey: 'site' });
    await settle();
    env.widgets[0].options['error-callback']('300030');
    assert.equal(env.widgets[0].removed, true);
    const asked = handle.token();
    await settle();
    assert.equal(env.widgets.length, 2);
    env.widgets[1].options.callback('second-try');
    assert.equal(await asked, 'second-try');
  },

  async 'a widget is drawn into the container the caller gave, and the container stays'() {
    const env = page();
    const mine = element('div');
    env.document.querySelector = (selector) => (selector === '#slot' ? mine : null);
    const byElement = env.api().token({ sitekey: 'site', container: mine });
    await settle();
    assert.equal(env.widgets[0].container, mine);
    env.widgets[0].options.callback('a');
    await byElement;
    const bySelector = env.api().token({ sitekey: 'site', container: '#slot' });
    await settle();
    assert.equal(env.widgets[1].container, mine);
    env.widgets[1].options.callback('b');
    await bySelector;
    assert.equal(env.widgets[1].removed, true);
    assert.equal(env.document.body.children.length, 0);
  },

  async 'the page is told when a person has to act'() {
    const env = page();
    const seen = [];
    const asked = env.api().token({ sitekey: 'site', onInteractive: (visible) => seen.push(visible) });
    await settle();
    env.widgets[0].options['before-interactive-callback']();
    env.widgets[0].options['after-interactive-callback']();
    env.widgets[0].options.callback('t');
    await asked;
    assert.deepEqual(seen, [true, false]);
  },

  async 'a callback that comes after the handle is over changes nothing'() {
    const env = page();
    const handle = env.api().prepare({ sitekey: 'site' });
    await settle();
    env.widgets[0].options.callback('t');
    await handle.token();
    env.widgets[0].options['error-callback']('late');
    env.widgets[0].options['expired-callback']();
    env.widgets[0].options.callback('again');
    assert.equal(env.widgets[0].resets, 0);
    assert.equal(env.widgets.length, 1);
  },

  async 'the behaviour of the widget is registered when arandu.ui is on the page'() {
    const env = page({ ui: true });
    assert.deepEqual(Object.keys(env.definitions), ['turnstile']);
    assert.equal(env.api().behavior, 'turnstile');
    assert.equal(typeof env.api().token, 'function');
  },

  async 'a page that draws the widget without arandu.ui is told, and still gets tokens'() {
    const env = page({ marked: true });
    assert.equal(env.errors.length, 1);
    assert.match(env.errors[0], /arandu\.ui is not on the page/);
    assert.deepEqual(env.definitions, {});
    const asked = env.api().token({ sitekey: 'site' });
    await settle();
    env.widgets[0].options.callback('t');
    assert.equal(await asked, 't');
  },

  async 'a page that only asks for tokens is not told anything'() {
    const env = page();
    assert.deepEqual(env.errors, []);
  },

  async 'the widget behaviour renders on mount, renews after a request and removes on destroy'() {
    const env = page({ ui: true });
    const hooks = env.definitions.turnstile;
    const part = element('div');
    const form = element('form');
    const ctx = {
      props: { sitekey: 'site', action: 'signup', theme: 'dark' },
      element: { querySelector: (selector) => (selector === '[data-part="widget"]' ? part : null), closest: (tag) => (tag === 'form' ? form : null) },
    };
    hooks.mounted(ctx);
    await settle();
    assert.equal(env.widgets.length, 1);
    assert.equal(env.widgets[0].container, part);
    assert.equal(env.widgets[0].options.sitekey, 'site');
    assert.equal(env.widgets[0].options.action, 'signup');
    assert.equal(env.widgets[0].options.theme, 'dark');
    assert.equal('callback' in env.widgets[0].options, false);
    assert.equal('appearance' in env.widgets[0].options, false);

    form.fire('ypy:finally:request');
    assert.equal(env.widgets[0].resets, 1);

    hooks.destroyed(ctx);
    assert.equal(env.widgets[0].removed, true);
    assert.equal((form.listeners['ypy:finally:request'] || []).length, 0);
  },

  async 'a widget destroyed before the script arrived is never drawn'() {
    const env = page({ ui: true });
    const hooks = env.definitions.turnstile;
    const ctx = {
      props: { sitekey: 'site' },
      element: { querySelector: () => element('div'), closest: () => null },
    };
    hooks.mounted(ctx);
    hooks.destroyed(ctx);
    await settle();
    assert.equal(env.widgets.length, 0);
  },

  async 'the widget behaviour does nothing without a site key'() {
    const env = page({ ui: true });
    env.definitions.turnstile.mounted({ props: {}, element: { querySelector: () => element('div'), closest: () => null } });
    await settle();
    assert.equal(env.scripts.length, 0);
  },
};

(async () => {
  let failed = false;
  for (const [name, scenario] of Object.entries(scenarios)) {
    let error = '';
    try {
      await scenario();
    } catch (caught) {
      error = String((caught && caught.stack) || caught);
      failed = true;
    }
    process.stdout.write(JSON.stringify({ name, error }) + '\n');
  }
  process.exit(failed ? 1 : 0);
})();
