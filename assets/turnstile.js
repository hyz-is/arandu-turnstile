/* The client half of Arandu Turnstile. It carries two things, and neither one
 * reads an attribute and evaluates it or writes anything inline, so a page
 * keeps script-src 'self' plus Cloudflare's one origin.
 *
 * 1. The "turnstile" behaviour of turnstile.Widget, registered with
 *    arandu.ui.define, so it runs on the element the component draws and
 *    nowhere else: mounted on the first load and on every swap that brings a
 *    widget in, destroyed when the swap takes it away.
 *
 * 2. arandu.turnstile.token and arandu.turnstile.prepare, for a page that has
 *    no form: a script that has to prove a person is in the browser before it
 *    sends a request of its own (a fetch, a payment step) asks for a token and
 *    sends it with that request. No component is needed, and no arandu.ui.
 *
 * Cloudflare's script is loaded once, the first time either one asks for it,
 * from the one address Cloudflare serves it at, with render=explicit: the
 * widget is drawn by this file, on the element it was given, and never by
 * Cloudflare's scan of the page -- a scan that runs once, on load, and misses
 * every widget a swap brings in afterwards.
 *
 * A token is single-use. When the form around a widget sends a request with
 * YPY and the widget is still on the page afterwards -- the form was refused
 * and only its messages were merged in -- the widget is reset, so the next
 * attempt carries a new token instead of the spent one. YPY fires
 * ypy:finally:request after every request it sent, whatever the answer was.
 * The element the widget draws into carries ypy-preserve="children": a morph,
 * which is what a swap does unless it names another style, would otherwise
 * empty it, because the answer holds that element with nothing in it.
 *
 * The component writes the tag that loads this file once per widget, so a page
 * with two widgets loads it twice. The second run finds arandu.turnstile
 * already defined and does nothing.
 */
(function () {
	'use strict';

	var arandu = window.arandu = window.arandu || {};
	if (arandu.turnstile) return;

	var SOURCE = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
	/* The options the caller and the component hand over, by the names
	 * turnstile.render takes. One that was not written is not passed, and
	 * Cloudflare's default stands. */
	var OPTIONS = ['action', 'cData', 'theme', 'size', 'language', 'appearance'];
	/* How long token() waits for a person or for Cloudflare, in milliseconds. */
	var WAIT = 60000;

	var loading = null;

	/* failure is the error every rejection of this file carries. code is what a
	 * caller branches on: challenge_unavailable (Cloudflare's script did not
	 * load), challenge_failed (the challenge errored or timed out; reason says
	 * which), challenge_unconfigured (no site key), challenge_cancelled and
	 * challenge_spent (the handle was cancelled or already gave its token). */
	function failure(code, reason) {
		var error = new Error('turnstile: ' + code + (reason ? ' (' + reason + ')' : ''));
		error.code = code;
		error.reason = reason || '';
		return error;
	}

	/* load resolves with Cloudflare's turnstile object once its script is on the
	 * page, adding the script the first time it is asked for. A load that failed
	 * is forgotten, so the next ask tries again. */
	function load() {
		if (window.turnstile && typeof window.turnstile.render === 'function') return Promise.resolve(window.turnstile);
		if (loading) return loading;

		loading = new Promise(function (resolve, reject) {
			var tag = document.createElement('script');
			tag.src = SOURCE;
			tag.async = true;
			tag.addEventListener('load', function () {
				if (window.turnstile && typeof window.turnstile.render === 'function') {
					resolve(window.turnstile);
					return;
				}
				loading = null;
				reject(failure('challenge_unavailable', 'no turnstile object after the script loaded'));
			});
			tag.addEventListener('error', function () {
				/* Refused by the policy is by far the likeliest reason, and the fix
				 * is on the server, so that is what the message names. */
				loading = null;
				if (window.console && console.error) {
					console.error('turnstile: ' + SOURCE + ' did not load. The route that draws this page has to be ' +
						'wrapped in turnstile.Headers, which allows https://challenges.cloudflare.com in script-src and frame-src');
				}
				reject(failure('challenge_unavailable', 'the script did not load'));
			});
			(document.head || document.documentElement).appendChild(tag);
		});
		return loading;
	}

	/* prepare starts solving a challenge now and keeps its token good until
	 * token() takes it. It returns a handle with two calls:
	 *
	 *   token()   a Promise of the token: at once when one is held, otherwise
	 *             when the challenge in progress is solved. The token is taken,
	 *             and the handle is spent -- a token proves one request.
	 *   cancel()  drops the challenge. A token() still waiting is rejected.
	 *
	 * While a token is held and nobody has taken it, the token expires after a
	 * few minutes; the widget is then reset and solves a new one, so token()
	 * never answers with an expired token.
	 *
	 * options: sitekey (required), action, cData, theme, size, language,
	 * appearance (default "interaction-only": nothing is drawn unless a person
	 * has to act), container (an element or a selector to draw into, default a
	 * div added to the body and removed afterwards), onInteractive(visible)
	 * (called with true when the challenge needs a person and false when it no
	 * longer does), timeout (milliseconds token() waits, default 60000, 0 for
	 * none). A hidden input is never created: the token is the Promise's value.
	 */
	function prepare(options) {
		options = options || {};
		if (!options.sitekey) throw failure('challenge_unconfigured', 'no sitekey');

		var slot = {
			/* solving, ready, failed or closed */
			state: 'solving',
			value: '',
			error: null,
			widget: undefined,
			holder: null,
			own: false,
			waiting: null,
			spent: false
		};

		function container() {
			var given = options.container;
			if (typeof given === 'string') given = document.querySelector(given);
			if (given) return given;
			var holder = document.createElement('div');
			holder.setAttribute('data-turnstile-token', '');
			(document.body || document.documentElement).appendChild(holder);
			slot.own = true;
			return holder;
		}

		/* detach removes the widget Cloudflare drew, and the element this file
		 * added for it. An element the caller gave stays. */
		function detach() {
			if (slot.widget !== undefined && window.turnstile) {
				try { window.turnstile.remove(slot.widget); } catch (error) { /* already gone */ }
			}
			slot.widget = undefined;
			if (slot.own && slot.holder && slot.holder.parentNode) slot.holder.parentNode.removeChild(slot.holder);
			slot.holder = null;
			slot.own = false;
		}

		function close() {
			slot.state = 'closed';
			slot.value = '';
			detach();
		}

		function finish(settle, value) {
			var waiting = slot.waiting;
			slot.waiting = null;
			if (waiting && waiting.timer) clearTimeout(waiting.timer);
			if (waiting) waiting[settle](value);
		}

		/* fail: a waiting token() is rejected and the handle is over. A handle
		 * nobody has asked yet keeps the failure and solves again when asked,
		 * because the reason may be gone by then. */
		function fail(error) {
			if (slot.state === 'closed') return;
			if (slot.waiting) {
				close();
				finish('reject', error);
				return;
			}
			detach();
			slot.state = 'failed';
			slot.error = error;
		}

		function solved(token) {
			if (slot.state === 'closed') return;
			slot.value = token;
			slot.state = 'ready';
			if (slot.waiting) deliver();
		}

		function deliver() {
			var value = slot.value;
			close();
			finish('resolve', value);
		}

		function start() {
			slot.state = 'solving';
			slot.value = '';
			slot.error = null;
			load().then(function (turnstile) {
				if (slot.state !== 'solving') return;
				var render = {
					sitekey: options.sitekey,
					appearance: 'interaction-only',
					'response-field': false,
					/* This file handles an error and an expiry itself. */
					retry: 'never',
					'refresh-expired': 'never',
					callback: solved,
					'error-callback': function (code) { fail(failure('challenge_failed', String(code || 'error'))); },
					'timeout-callback': function () { fail(failure('challenge_failed', 'timeout')); },
					'unsupported-callback': function () { fail(failure('challenge_failed', 'unsupported')); },
					'expired-callback': function () {
						if (slot.state !== 'ready' || slot.widget === undefined) return;
						slot.state = 'solving';
						slot.value = '';
						turnstile.reset(slot.widget);
					}
				};
				for (var i = 0; i < OPTIONS.length; i++) {
					if (options[OPTIONS[i]]) render[OPTIONS[i]] = options[OPTIONS[i]];
				}
				if (typeof options.onInteractive === 'function') {
					render['before-interactive-callback'] = function () { options.onInteractive(true); };
					render['after-interactive-callback'] = function () { options.onInteractive(false); };
				}
				slot.holder = container();
				slot.widget = turnstile.render(slot.holder, render);
			}, function (error) {
				fail(error);
			});
		}

		start();

		return {
			token: function () {
				return new Promise(function (resolve, reject) {
					if (slot.spent) {
						reject(failure('challenge_spent', 'a handle gives one token'));
						return;
					}
					if (slot.state === 'closed') {
						reject(failure('challenge_cancelled', 'the challenge was cancelled'));
						return;
					}
					slot.spent = true;
					slot.waiting = { resolve: resolve, reject: reject, timer: null };
					if (slot.state === 'ready') {
						deliver();
						return;
					}
					if (slot.state === 'failed') start();
					var wait = options.timeout === undefined ? WAIT : options.timeout;
					if (wait > 0 && slot.waiting) {
						slot.waiting.timer = setTimeout(function () { fail(failure('challenge_failed', 'timeout')); }, wait);
					}
				});
			},
			cancel: function () {
				if (slot.state === 'closed') return;
				close();
				finish('reject', failure('challenge_cancelled', 'the challenge was cancelled'));
			}
		};
	}

	/* token solves one challenge and resolves with its token: prepare and
	 * token() in one call, for a script that needs the token now. It never
	 * throws; a missing site key is a rejection like every other. */
	function token(options) {
		try {
			return prepare(options).token();
		} catch (error) {
			return Promise.reject(error);
		}
	}

	arandu.turnstile = { behavior: 'turnstile', token: token, prepare: prepare };

	if (!arandu.ui || typeof arandu.ui.define !== 'function') {
		/* A page that only asks for tokens needs no arandu.ui. One that drew the
		 * widget does, and is told. */
		if (document.querySelector('[data-kyse-behavior="turnstile"]') && window.console && console.error) {
			console.error('turnstile: arandu.ui is not on the page; the layout has to load ui.js before this script');
		}
		return;
	}

	arandu.ui.define('turnstile', {
		mounted: function (ctx) {
			var props = ctx.props || {};
			var target = ctx.element.querySelector('[data-part="widget"]');
			if (!target || !props.sitekey) return;

			load().then(function (turnstile) {
				if (ctx.gone) return;
				var options = { sitekey: props.sitekey };
				for (var i = 0; i < OPTIONS.length; i++) {
					if (props[OPTIONS[i]]) options[OPTIONS[i]] = props[OPTIONS[i]];
				}
				ctx.widget = turnstile.render(target, options);
			}, function () { /* load has said why on the console */ });

			var form = ctx.element.closest('form');
			if (!form) return;
			ctx.form = form;
			ctx.renew = function () {
				if (ctx.gone || ctx.widget === undefined || !window.turnstile) return;
				if (!document.documentElement.contains(ctx.element)) return;
				window.turnstile.reset(ctx.widget);
			};
			form.addEventListener('ypy:finally:request', ctx.renew);
		},
		destroyed: function (ctx) {
			ctx.gone = true;
			if (ctx.form && ctx.renew) ctx.form.removeEventListener('ypy:finally:request', ctx.renew);
			if (ctx.widget !== undefined && window.turnstile) window.turnstile.remove(ctx.widget);
		},
	});
})();
