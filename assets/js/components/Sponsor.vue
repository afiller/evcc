<template>
	<div v-if="sponsor">
		<p class="mb-3">
			{{ $t("footer.sponsor.thanks", { sponsor }) }}
		</p>
		<div class="d-flex justify-content-center align-items-center flex-column">
			<button
				ref="confetti"
				class="btn btn btn-outline-primary mb-2 confetti-button bg-evcc w-75 rounded"
				@click="surprise"
			>
				<shopicon-regular-heart class="me-1 d-inline-block"></shopicon-regular-heart>
				{{ $t("footer.sponsor.confetti") }}
			</button>
			<a href="https://evcc.io/sticker" target="_blank" class="small text-muted">
				{{ $t("footer.sponsor.sticker") }}
			</a>
		</div>
	</div>
	<div v-else>
		<p class="mb-3">
			{{ $t("footer.sponsor.supportUs") }}
		</p>
		<div class="d-flex justify-content-center align-items-center flex-column">
			<a
				target="_blank"
				href="https://github.com/sponsors/andig"
				class="btn btn-outline-primary mb-2 w-75"
			>
				<shopicon-regular-heart class="me-1 d-inline-block"></shopicon-regular-heart>
				{{ $t("footer.sponsor.becomeSponsor") }}
			</a>
			<div class="small text-muted text-center">
				{{ $t("footer.sponsor.confettiPromise") }}
			</div>
		</div>
	</div>
</template>

<script>
import confetti from "canvas-confetti";
import "@h2d2/shopicons/es/regular/heart";
import "@h2d2/shopicons/es/filled/heart";

export default {
	name: "Sponsor",
	props: {
		sponsor: String,
	},
	methods: {
		surprise: function () {
			const $el = this.$refs.confetti;
			const angle = 45 + Math.random() * 90;
			const drift = 0;

			const { top, height, left, width } = $el.getBoundingClientRect();
			const x = (left + width / 2) / window.innerWidth;
			const y = (top + height / 2) / window.innerHeight;
			const origin = { x, y };

			confetti({
				origin,
				angle,
				particleCount: 75 + Math.random() * 50,
				spread: 50 + Math.random() * 50,
				drift,
				scalar: 1.3,
				zIndex: 1056, // Bootstrap Modal is 1055
				colors: [
					"#0d6efd",
					"#0fdd42",
					"#408458",
					"#4923BA",
					"#5BC8EC",
					"#C54482",
					"#CC444A",
					"#EE8437",
					"#F7C144",
					"#FFFD54",
				],
			});
		},
	},
};
</script>

<style scoped>
.confetti-button {
	/* prevent double-tap zoom */
	touch-action: none;
	user-select: none;
}
</style>
