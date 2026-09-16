(function () {

	const isCallbackPage = window.location.pathname === '/oidc/callback';

	if (isCallbackPage) {
		// OIDC tab: signal the opener that the login finished and close the tab
		if (window.opener) {
			window.opener.postMessage('oidclogin', window.location.origin);
			window.close();
			return;
		}
		// No opener, redirect home
		window.location.href = '/';
		return;
	}

	// Login page: open the OIDC flow in a new tab and restore the original request after it finished

	function restoreOriginalRequest() {
		const loginForm = document.getElementById('loginform');
		if (!loginForm) {
			window.location.reload();
			return;
		}
		// Change the form to act as a restore request and submit it
		const actionInput = loginForm.querySelector('[name=loginaction]');
		if (actionInput) actionInput.value = 'restore';
		loginForm.querySelectorAll('input[required]').forEach(function (input) {
			input.disabled = true;
		});
		loginForm.submit();
	}

	const loginButton = document.getElementById('loginoidcbutton');
	if (!loginButton) return;

	loginButton.addEventListener('click', function (event) {
		event.preventDefault();
		const oidcTab = window.open('/oidc/login', 'oidclogin');
		if (!oidcTab) {
			// Tab could not be opened, fallback to redirect flow
			document.getElementById('oidcloginform').submit();
			return;
		}
		window.addEventListener('message', function (event) {
			if (event.origin !== window.location.origin) return;
			if (event.data !== 'oidclogin') return;
			restoreOriginalRequest();
		});
	});

})();
