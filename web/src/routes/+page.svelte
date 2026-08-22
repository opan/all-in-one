<script lang="ts">
	import ThemeToggle from '../components/theme-toggle.svelte';

	const APP_URL = '/home';
	const GITHUB_URL = 'https://github.com/opan/all-in-one';
	const ISSUES_URL = 'https://github.com/opan/all-in-one/issues';

	let navOpen = $state(false);

	// Fade + slide elements in as they scroll into view. Honors reduced-motion
	// and degrades to "visible" when IntersectionObserver isn't available.
	function reveal(node: HTMLElement, params: { delay?: number } = {}) {
		const reduce =
			typeof window === 'undefined' ||
			typeof IntersectionObserver === 'undefined' ||
			window.matchMedia('(prefers-reduced-motion: reduce)').matches;
		if (reduce) {
			node.classList.add('in');
			return;
		}
		if (params.delay) node.style.transitionDelay = `${params.delay}ms`;
		const io = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting) {
						node.classList.add('in');
						io.unobserve(node);
					}
				}
			},
			{ threshold: 0.15, rootMargin: '0px 0px -8% 0px' }
		);
		io.observe(node);
		return {
			destroy() {
				io.disconnect();
			}
		};
	}

	const features = [
		{
			kicker: 'Topics & items',
			title: 'Listing',
			body: "I use it to track the books I've bought and read. Spin up a topic, shape it with a JSON schema, and manage the items in one place — see if it covers your use case too.",
			image: '/landing/listing.png',
			alt: 'Listing topics table'
		},
		{
			kicker: 'WebSockets',
			title: 'Real-time Chat',
			body: "My own alternative for reaching people — real-time messaging with sessions and invites. The conversations live on my personal server, so they stay private.",
			image: '/landing/chat.png',
			alt: 'Real-time chat conversations'
		},
		{
			kicker: 'Short links',
			title: 'Shortener',
			body: "When a link is too long to share, I shorten it here instead of hunting down yet another online tool. Long URLs become short, trackable links.",
			image: '/landing/shortener.png',
			alt: 'URL shortener links'
		}
	];

	// Features carousel — one slide at a time, manual (no autoplay), loops around.
	let current = $state(0);
	const total = features.length;
	const goTo = (i: number) => (current = ((i % total) + total) % total);
	const next = () => goTo(current + 1);
	const prev = () => goTo(current - 1);

	// Touch swipe on the carousel viewport (mobile).
	let touchStartX = 0;
	const onTouchStart = (e: TouchEvent) => (touchStartX = e.changedTouches[0].clientX);
	const onTouchEnd = (e: TouchEvent) => {
		const dx = e.changedTouches[0].clientX - touchStartX;
		if (Math.abs(dx) > 40) (dx < 0 ? next : prev)();
	};
</script>

<svelte:head>
	<title>All-in-One — a full-stack app you can try right now</title>
	<meta
		name="description"
		content="All-in-One bundles listing, real-time chat and a live rate limiter into one open-source Go + Svelte project. Try it live, or clone the code and make it your own."
	/>
</svelte:head>

<div class="landing">
	<!-- Nav (borderless, seamless with hero) -->
	<header class="nav">
		<span class="brand">ALL-IN-ONE</span>
		<nav class="nav-links" class:open={navOpen}>
			<a href="#features" onclick={() => (navOpen = false)}>Features</a>
			<a href="#features" onclick={() => (navOpen = false)}>Listing</a>
			<a href="#features" onclick={() => (navOpen = false)}>Chat</a>
			<a href={GITHUB_URL} target="_blank" rel="noreferrer" onclick={() => (navOpen = false)}>GitHub</a>
		</nav>
		<div class="nav-actions">
			<ThemeToggle />
			<a class="btn btn-solid btn-sm nav-cta" href={APP_URL}>Try it here</a>
			<button
				class="nav-toggle"
				aria-label="Toggle menu"
				aria-expanded={navOpen}
				onclick={() => (navOpen = !navOpen)}
			>
				<span></span><span></span><span></span>
			</button>
		</div>
	</header>

	<!-- Hero (full-bleed, no borders; screenshot floats on the page) -->
	<section class="hero">
		<div class="hero-copy reveal" use:reveal>
			<div class="kicker">Open Source / Go + Svelte</div>
			<h1>I built this to solve my problems. See if it solves yours.</h1>
			<p class="lead">
				All-in-One is the super-app I'm building to fix my own everyday problems — listing,
				real-time chat, short links and more, all in one place. It's live right now, so try it and
				see if it solves yours too.
			</p>
			<div class="btn-row">
				<a class="btn btn-solid" href={APP_URL}>Try it here</a>
				<a class="btn btn-outline" href={GITHUB_URL} target="_blank" rel="noreferrer">
					View on GitHub
				</a>
			</div>
		</div>
		<div class="hero-media reveal" use:reveal={{ delay: 120 }}>
			<img src="/landing/dashboard.png" alt="All-in-One dashboard" loading="eager" />
		</div>
	</section>

	<!-- Features — carousel, one slide at a time -->
	<section id="features" class="features-intro reveal" use:reveal>
		<div class="kicker">What's inside</div>
		<h2>Three main features now — more to come.</h2>
	</section>
	<section
		class="carousel reveal"
		use:reveal
		aria-roledescription="carousel"
		aria-label="Product features"
	>
		<button class="car-arrow prev" onclick={prev} aria-label="Previous feature" type="button">
			<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M15 5l-7 7 7 7" /></svg>
		</button>

		<div
			class="viewport"
			role="group"
			aria-label="Feature slides"
			ontouchstart={onTouchStart}
			ontouchend={onTouchEnd}
		>
			<div class="track" style="transform: translateX(-{current * 100}%)">
				{#each features as feature, i (feature.title)}
					<div
						class="slide"
						role="group"
						aria-roledescription="slide"
						aria-label={`${i + 1} of ${total}: ${feature.title}`}
						aria-hidden={i !== current}
					>
						<div class="slide-media">
							<img src={feature.image} alt={feature.alt} loading={i === 0 ? 'eager' : 'lazy'} />
						</div>
						<div class="slide-copy">
							<div class="kicker">{feature.kicker}</div>
							<h3>{feature.title}</h3>
							<p>{feature.body}</p>
							<a class="btn btn-outline btn-sm" href={APP_URL}>Open {feature.title}</a>
						</div>
					</div>
				{/each}
			</div>
		</div>

		<button class="car-arrow next" onclick={next} aria-label="Next feature" type="button">
			<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 5l7 7-7 7" /></svg>
		</button>

		<div class="dots" aria-label="Choose feature">
			{#each features as feature, i (feature.title)}
				<button
					class="dot"
					class:active={i === current}
					onclick={() => goTo(i)}
					aria-current={i === current}
					aria-label={`Show ${feature.title}`}
					type="button"
				></button>
			{/each}
		</div>
	</section>

	<!-- Security — separate band highlighting the built-in 2FA -->
	<section id="security" class="security reveal" use:reveal>
		<div class="security-copy">
			<div class="kicker">Account security</div>
			<h2>Security that's built in, not bolted on.</h2>
			<p>
				Turn on two-factor authentication straight from your settings. Scan the QR code with any
				authenticator app, and every sign-in asks for a time-based code — with one-time recovery
				codes as your backup if you lose the device.
			</p>
			<ul class="security-points">
				<li>TOTP two-factor auth, opt-in per account</li>
				<li>Downloadable recovery codes you can regenerate anytime</li>
				<li>Disable it just as easily whenever you want</li>
			</ul>
			<a class="btn btn-solid" href={APP_URL}>Try it here</a>
		</div>
		<div class="security-media">
			<img src="/landing/twofa.png" alt="Two-factor authentication settings" loading="lazy" />
		</div>
	</section>

	<!-- Closing banner -->
	<section class="banner reveal" use:reveal>
		<h2>Like this app? Try it here — and if it's for you, install your own copy.</h2>
		<div class="btn-row">
			<a class="btn btn-on-accent" href={APP_URL}>Try it here</a>
			<a class="btn btn-outline-accent" href={ISSUES_URL} target="_blank" rel="noreferrer">
				Report an issue
			</a>
		</div>
	</section>

	<!-- Footer (no top border) -->
	<footer class="footer">
		<span class="footer-brand">ALL-IN-ONE</span>
		<span class="footer-note">Built by opan / Open source</span>
		<div class="tags">
			<span class="tag">Go</span>
			<span class="tag">Svelte</span>
			<span class="tag">TypeScript</span>
			<span class="tag">SQLite</span>
		</div>
	</footer>
</div>

<style>
	.landing {
		/* Modernist Blue — light (1c) */
		--bg: #f4f7ff;
		--text: #17203a;
		--muted: rgba(23, 32, 58, 0.8);
		--faint: rgba(23, 32, 58, 0.55);
		--kicker: #1f56d6;
		--nav-link: #17203a;
		--line: rgba(23, 32, 58, 0.3);
		--outline: rgba(23, 32, 58, 0.3);
		--outline-hover: rgba(23, 32, 58, 0.06);
		--accent: #3b7bff;
		--accent-hover: #2f6bff;
		--media-radius: 0px;
		--shadow: 0 18px 40px -16px rgba(23, 32, 58, 0.32);
		--shadow-hover: 0 30px 60px -20px rgba(23, 32, 58, 0.4);
		--glow: rgba(59, 123, 255, 0.14);

		min-height: 100vh;
		background: var(--bg);
		color: var(--text);
		font-family: 'Archivo', system-ui, sans-serif;
		font-size: 16px;
		line-height: 1.5;
	}

	:global(.dark) .landing {
		/* Modernist Blue — dark (1d) */
		--bg: #0d1526;
		--text: #e8edf7;
		--muted: rgba(232, 237, 247, 0.72);
		--faint: rgba(232, 237, 247, 0.45);
		--kicker: #7aa6ff;
		--nav-link: #c7d2e8;
		--line: rgba(255, 255, 255, 0.14);
		--outline: rgba(255, 255, 255, 0.24);
		--outline-hover: rgba(255, 255, 255, 0.08);
		--accent: #3b7bff;
		--accent-hover: #5a90ff;
		--shadow: 0 18px 40px -16px rgba(0, 0, 0, 0.55);
		--shadow-hover: 0 30px 60px -20px rgba(0, 0, 0, 0.65);
		--glow: rgba(59, 123, 255, 0.22);
	}

	:global(html) {
		scroll-behavior: smooth;
	}

	/* Scroll-reveal: elements fade + rise as they enter the viewport. */
	.reveal {
		opacity: 0;
		transform: translateY(28px);
		transition:
			opacity 0.7s cubic-bezier(0.22, 0.61, 0.36, 1),
			transform 0.7s cubic-bezier(0.22, 0.61, 0.36, 1);
		will-change: opacity, transform;
	}
	.reveal:global(.in) {
		opacity: 1;
		transform: none;
	}
	@media (prefers-reduced-motion: reduce) {
		.reveal {
			opacity: 1;
			transform: none;
			transition: none;
		}
	}

	/* Only reset decoration here; every anchor sets its own color below. A color
	   reset would win over the button color rules on specificity and blank them out. */
	.landing :global(a) {
		text-decoration: none;
	}

	/* Shared building blocks */
	.kicker {
		font-size: 11px;
		font-weight: 600;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--kicker);
	}

	.btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		font-weight: 800;
		font-size: 15px;
		line-height: 1;
		padding: 13px 24px;
		border: 2px solid transparent;
		cursor: pointer;
		transition:
			background 0.15s ease,
			opacity 0.15s ease;
	}

	.btn-sm {
		font-size: 14px;
		padding: 9px 18px;
	}

	.btn-solid {
		background: var(--accent);
		color: #ffffff;
	}
	.btn-solid:hover {
		background: var(--accent-hover);
	}

	.btn-outline {
		color: var(--text);
		border-color: var(--outline);
	}
	.btn-outline:hover {
		background: var(--outline-hover);
	}

	.btn-on-accent {
		background: #ffffff;
		color: #2f6bff;
	}
	.btn-on-accent:hover {
		opacity: 0.9;
	}

	.btn-outline-accent {
		color: #ffffff;
		border-color: #ffffff;
	}
	.btn-outline-accent:hover {
		background: rgba(255, 255, 255, 0.16);
	}

	.btn-row {
		display: flex;
		flex-wrap: wrap;
		gap: 12px;
	}

	/* Nav — no border, seamless with the hero */
	.nav {
		position: relative;
		display: flex;
		align-items: center;
		gap: 28px;
		padding: 16px 44px;
	}
	.brand {
		font-weight: 800;
		font-size: 19px;
		letter-spacing: -0.02em;
		margin-right: auto;
		white-space: nowrap;
	}
	.nav-links {
		display: flex;
		align-items: center;
		gap: 28px;
	}
	.nav-links a {
		font-size: 14px;
		font-weight: 500;
		color: var(--nav-link);
	}
	.nav-links a:hover {
		color: var(--accent);
	}
	.nav-actions {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.nav-toggle {
		display: none;
		flex-direction: column;
		gap: 5px;
		width: 40px;
		height: 40px;
		align-items: center;
		justify-content: center;
		background: transparent;
		border: none;
		padding: 0;
		cursor: pointer;
	}
	.nav-toggle span {
		display: block;
		width: 22px;
		height: 2px;
		background: var(--text);
	}

	/* Hero — full-bleed, transparent screenshot slot */
	.hero {
		position: relative;
		display: grid;
		grid-template-columns: 1.05fr 0.95fr;
		gap: 0;
	}
	/* Soft accent glow behind the hero screenshot for depth. */
	.hero::before {
		content: '';
		position: absolute;
		inset: 0;
		z-index: 0;
		pointer-events: none;
		background: radial-gradient(46% 62% at 78% 30%, var(--glow), transparent 72%);
	}
	.hero-copy,
	.hero-media {
		position: relative;
		z-index: 1;
	}
	.hero-copy {
		padding: 60px 44px;
	}
	.hero-copy .kicker {
		margin-bottom: 20px;
	}
	.hero h1 {
		font-weight: 800;
		font-size: 56px;
		line-height: 1;
		letter-spacing: -0.03em;
		margin: 0 0 22px;
	}
	.lead {
		font-size: 17px;
		line-height: 1.5;
		color: var(--muted);
		max-width: 34em;
		margin: 0 0 30px;
	}
	.hero-media {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 340px;
		padding: 32px 44px 32px 0;
	}
	.hero-media img {
		width: 100%;
		height: auto;
		display: block;
		border-radius: var(--media-radius);
		box-shadow: var(--shadow);
		transition:
			transform 0.5s cubic-bezier(0.22, 0.61, 0.36, 1),
			box-shadow 0.5s ease;
	}
	.hero-media:hover img {
		transform: translateY(-6px);
		box-shadow: var(--shadow-hover);
	}
	@media (prefers-reduced-motion: no-preference) {
		.hero-media img {
			animation: float 7s ease-in-out infinite;
		}
		.hero-media:hover img {
			animation-play-state: paused;
		}
	}
	@keyframes float {
		0%,
		100% {
			transform: translateY(0);
		}
		50% {
			transform: translateY(-10px);
		}
	}

	/* Features */
	.features-intro {
		padding: 52px 44px 0;
	}
	.features-intro .kicker {
		margin-bottom: 8px;
	}
	.features-intro h2 {
		font-weight: 800;
		font-size: 34px;
		letter-spacing: -0.02em;
		margin: 0 0 28px;
	}
	/* Features carousel */
	.carousel {
		position: relative;
		padding: 0 44px 56px;
	}
	.viewport {
		overflow: hidden;
	}
	.track {
		display: flex;
		transition: transform 0.5s cubic-bezier(0.22, 0.61, 0.36, 1);
		will-change: transform;
	}
	.slide {
		flex: 0 0 100%;
		min-width: 0;
		display: grid;
		grid-template-columns: 1.35fr 1fr;
		gap: 44px;
		align-items: center;
		padding: 6px 2px 20px;
	}
	.slide[aria-hidden='true'] {
		visibility: hidden;
	}
	.slide-media img {
		width: 100%;
		height: auto;
		display: block;
		border-radius: var(--media-radius);
		box-shadow: var(--shadow);
	}
	.slide-copy {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 14px;
	}
	.slide-copy .kicker {
		margin: 0;
	}
	.slide-copy h3 {
		font-weight: 800;
		font-size: 28px;
		line-height: 1.05;
		letter-spacing: -0.02em;
		margin: 0;
	}
	.slide-copy p {
		margin: 0;
		font-size: 15px;
		line-height: 1.55;
		color: var(--muted);
		max-width: 34em;
	}
	.slide-copy .btn {
		margin-top: 4px;
	}

	.car-arrow {
		position: absolute;
		top: 38%;
		z-index: 2;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 44px;
		background: var(--bg);
		border: 2px solid var(--outline);
		color: var(--text);
		cursor: pointer;
		transition:
			background 0.15s ease,
			border-color 0.15s ease;
	}
	.car-arrow:hover {
		background: var(--outline-hover);
		border-color: var(--accent);
	}
	.car-arrow.prev {
		left: 4px;
	}
	.car-arrow.next {
		right: 4px;
	}
	.car-arrow svg {
		width: 22px;
		height: 22px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2.5;
		stroke-linecap: round;
		stroke-linejoin: round;
	}
	.dots {
		display: flex;
		justify-content: center;
		gap: 10px;
	}
	.dot {
		width: 28px;
		height: 4px;
		background: var(--line);
		border: none;
		padding: 0;
		cursor: pointer;
		transition: background 0.2s ease;
	}
	.dot.active {
		background: var(--accent);
	}
	@media (prefers-reduced-motion: reduce) {
		.track {
			transition: none;
		}
	}

	/* Security band */
	.security {
		display: grid;
		grid-template-columns: 1fr 1.1fr;
		gap: 48px;
		align-items: center;
		padding: 64px 44px;
		background: color-mix(in srgb, var(--accent) 6%, var(--bg));
	}
	.security-copy {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 16px;
	}
	.security-copy h2 {
		font-weight: 800;
		font-size: 34px;
		line-height: 1.05;
		letter-spacing: -0.02em;
		margin: 0;
	}
	.security-copy > p {
		margin: 0;
		font-size: 15px;
		line-height: 1.6;
		color: var(--muted);
		max-width: 36em;
	}
	.security-points {
		list-style: none;
		padding: 0;
		margin: 2px 0;
		display: flex;
		flex-direction: column;
		gap: 9px;
	}
	.security-points li {
		position: relative;
		padding-left: 26px;
		font-size: 14.5px;
		color: var(--text);
	}
	.security-points li::before {
		content: '';
		position: absolute;
		left: 2px;
		top: 5px;
		width: 11px;
		height: 6px;
		border-left: 2px solid var(--accent);
		border-bottom: 2px solid var(--accent);
		transform: rotate(-45deg);
	}
	.security-copy .btn {
		margin-top: 6px;
	}
	.security-media img {
		width: 100%;
		height: auto;
		display: block;
		border-radius: var(--media-radius);
		box-shadow: var(--shadow);
	}

	/* Banner */
	.banner {
		background: var(--accent);
		color: #ffffff;
		padding: 56px 44px;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 22px;
	}
	.banner h2 {
		font-weight: 800;
		font-size: 46px;
		line-height: 0.98;
		letter-spacing: -0.03em;
		margin: 0;
		max-width: 16em;
		color: #ffffff;
	}

	/* Footer — no top border */
	.footer {
		padding: 26px 44px;
		display: flex;
		align-items: center;
		gap: 16px;
		flex-wrap: wrap;
	}
	.footer-brand {
		font-weight: 800;
		font-size: 16px;
	}
	.footer-note {
		font-size: 13px;
		color: var(--faint);
	}
	.tags {
		margin-left: auto;
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}
	.tag {
		border: 1px solid var(--outline);
		color: var(--text);
		font-size: 11px;
		font-weight: 600;
		padding: 3px 10px;
	}

	/* Responsive — below ~900px the diagonal collapses to one column */
	@media (max-width: 900px) {
		.nav {
			padding: 14px 20px;
		}
		.nav-links {
			position: absolute;
			top: 100%;
			left: 0;
			right: 0;
			z-index: 20;
			display: none;
			flex-direction: column;
			align-items: flex-start;
			gap: 6px;
			padding: 12px 20px 20px;
			background: var(--bg);
			border-bottom: 2px solid var(--line);
		}
		.nav-links.open {
			display: flex;
		}
		.nav-toggle {
			display: inline-flex;
		}
		.nav-cta {
			display: none;
		}

		.hero {
			grid-template-columns: 1fr;
		}
		.hero-copy {
			padding: 36px 20px 20px;
		}
		.hero h1 {
			font-size: 40px;
		}
		.hero-media {
			min-height: 0;
			padding: 0 20px 32px;
		}

		.features-intro {
			padding: 32px 20px 0;
		}
		.carousel {
			padding: 0 20px 44px;
		}
		.slide {
			grid-template-columns: 1fr;
			gap: 20px;
			padding: 6px 0 16px;
		}
		.slide-copy h3 {
			font-size: 24px;
		}
		.car-arrow {
			top: 32%;
			width: 40px;
			height: 40px;
		}
		.car-arrow.prev {
			left: 0;
		}
		.car-arrow.next {
			right: 0;
		}

		.security {
			grid-template-columns: 1fr;
			gap: 28px;
			padding: 44px 20px;
		}
		.security-copy h2 {
			font-size: 28px;
		}

		.banner {
			padding: 40px 20px;
		}
		.banner h2 {
			font-size: 32px;
		}
		.footer {
			padding: 24px 20px;
		}
	}
</style>
