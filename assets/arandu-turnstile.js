/* The client half of turnstile.Widget.
 *
 * It registers one behaviour, "turnstile", with arandu.ui.define, so it runs
 * on the element the component draws and nowhere else: mounted on the first
 * load and on every swap that brings a widget in, destroyed when the swap takes
 * it away. Nothing here reads an attribute and evaluates it, and nothing is
 * inline, so the page keeps script-src 'self' plus Cloudflare's one origin.
 *
 * Cloudflare's script is loaded once, the first time a widget mounts, from the
 * one address Cloudflare serves it at, with render=explicit: the widget is
 * drawn by this file, on the element it was given, and never by Cloudflare's
 * scan of the page -- a scan that runs once, on load, and misses every widget
 * a swap brings in afterwards.
 *
 * A token is single-use. When the form around a widget sends a request with
 * htmx and the widget is still on the page afterwards -- the form was refused
 * and only its messages were swapped -- the widget is reset, so the next
 * attempt carries a new token instead of the spent one.
 *
 * The component writes the tag that loads this file once per widget, so a page
 * with two widgets loads it twice. The second run finds the behaviour already
 * registered and does nothing.
 */
(function () {
	'use strict';

	var arandu = window.arandu;
	if (!arandu || !arandu.ui || typeof arandu.ui.define !== 'function') {
		if (window.console && console.error) {
			console.error('arandu-turnstile: arandu.ui is not on the page; the layout has to load ui.js before this script');
		}
		return;
	}
	if (arandu.turnstile) return;

	var SOURCE = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
	/* The props the component hands over, by the names turnstile.render takes.
	 * A prop that was not written is not passed, and Cloudflare's default
	 * stands. */
	var OPTIONS = ['action', 'cData', 'theme', 'size', 'language', 'appearance'];

	var waiting = [];
	var loading = false;

	/* withTurnstile runs fn once Cloudflare's script is on the page, loading it
	 * the first time it is asked for. */
	function withTurnstile(fn) {
		if (window.turnstile && typeof window.turnstile.render === 'function') {
			fn(window.turnstile);
			return;
		}
		waiting.push(fn);
		if (loading) return;
		loading = true;

		var tag = document.createElement('script');
		tag.src = SOURCE;
		tag.async = true;
		tag.addEventListener('load', function () {
			var queued = waiting;
			waiting = [];
			for (var i = 0; i < queued.length; i++) queued[i](window.turnstile);
		});
		tag.addEventListener('error', function () {
			/* Refused by the policy is by far the likeliest reason, and the fix
			 * is on the server, so that is what the message names. */
			loading = false;
			waiting = [];
			if (window.console && console.error) {
				console.error('arandu-turnstile: ' + SOURCE + ' did not load. The route that draws this page has to be ' +
					'wrapped in turnstile.Headers, which allows https://challenges.cloudflare.com in script-src and frame-src');
			}
		});
		document.head.appendChild(tag);
	}

	arandu.turnstile = { behavior: 'turnstile' };

	arandu.ui.define('turnstile', {
		mounted: function (ctx) {
			var props = ctx.props || {};
			var target = ctx.element.querySelector('[data-part="widget"]');
			if (!target || !props.sitekey) return;

			withTurnstile(function (turnstile) {
				if (ctx.gone) return;
				var options = { sitekey: props.sitekey };
				for (var i = 0; i < OPTIONS.length; i++) {
					if (props[OPTIONS[i]]) options[OPTIONS[i]] = props[OPTIONS[i]];
				}
				ctx.widget = turnstile.render(target, options);
			});

			var form = ctx.element.closest('form');
			if (!form) return;
			ctx.form = form;
			ctx.renew = function () {
				if (ctx.gone || ctx.widget === undefined || !window.turnstile) return;
				if (!document.documentElement.contains(ctx.element)) return;
				window.turnstile.reset(ctx.widget);
			};
			form.addEventListener('htmx:afterRequest', ctx.renew);
		},
		destroyed: function (ctx) {
			ctx.gone = true;
			if (ctx.form && ctx.renew) ctx.form.removeEventListener('htmx:afterRequest', ctx.renew);
			if (ctx.widget !== undefined && window.turnstile) window.turnstile.remove(ctx.widget);
		},
	});
})();
