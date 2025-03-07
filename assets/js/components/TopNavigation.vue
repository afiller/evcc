<template>
	<div>
		<button
			type="button"
			data-bs-toggle="dropdown"
			data-bs-target="#navbarNavAltMarkup"
			aria-controls="navbarNavAltMarkup"
			aria-expanded="false"
			aria-label="Toggle navigation"
			class="btn btn-sm btn-outline-secondary position-relative"
		>
			<span
				v-if="logoutCount > 0"
				class="position-absolute top-0 start-100 translate-middle p-2 bg-danger border border-light rounded-circle"
			>
				<span class="visually-hidden">login available</span>
			</span>
			<shopicon-regular-menu></shopicon-regular-menu>
		</button>
		<ul class="dropdown-menu dropdown-menu-end">
			<li>
				<a class="dropdown-item" href="https://docs.evcc.io/blog/" target="_blank">
					{{ $t("header.blog") }}
				</a>
			</li>
			<li>
				<a class="dropdown-item" href="https://docs.evcc.io/docs/Home/" target="_blank">
					{{ $t("header.docs") }}
				</a>
			</li>
			<li>
				<a class="dropdown-item" href="https://github.com/evcc-io/evcc" target="_blank">
					{{ $t("header.github") }}
				</a>
			</li>
			<li>
				<a class="dropdown-item" href="https://evcc.io/" target="_blank">
					{{ $t("header.about") }}
				</a>
			</li>
			<template v-if="providerLogins.length > 0">
				<li><hr class="dropdown-divider" /></li>
				<li>
					<h6 class="dropdown-header">{{ $t("header.login") }}</h6>
				</li>
				<li v-for="login in providerLogins" :key="login.title">
					<button
						type="button"
						class="dropdown-item"
						@click="handleProviderAuthorization(login)"
					>
						<span
							v-if="!login.loggedIn"
							class="d-inline-block p-1 rounded-circle bg-danger border border-light rounded-circle"
						></span>
						{{ login.title }}
						{{ $t(login.loggedIn ? "main.provider.logout" : "main.provider.login") }}
					</button>
				</li>
			</template>
		</ul>
	</div>
</template>

<script>
import "@h2d2/shopicons/es/regular/menu";
import baseAPI from "../baseapi";

export default {
	name: "TopNavigation",
	props: {
		vehicleLogins: {
			type: Object,
			default: () => {
				return {};
			},
		},
	},
	computed: {
		logoutCount() {
			return this.providerLogins.filter((login) => !login.loggedIn).length;
		},
		providerLogins() {
			return Object.entries(this.vehicleLogins).map(([k, v]) => ({
				title: k,
				loggedIn: v.authenticated,
				loginPath: v.uri + "/login",
				logoutPath: v.uri + "/logout",
			}));
		},
	},
	methods: {
		handleProviderAuthorization: async function (provider) {
			if (!provider.loggedIn) {
				baseAPI.post(provider.loginPath).then(function (response) {
					window.location.href = response.data.loginUri;
				});
			} else {
				baseAPI.post(provider.logoutPath);
			}
		},
	},
};
</script>
