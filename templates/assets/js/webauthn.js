(function () {

    function isWebAuthnSupported() {
        return (
            !!(navigator.credentials &&
                typeof navigator.credentials.create === 'function' &&
                typeof navigator.credentials.get === 'function' &&
                window.PublicKeyCredential)
        );
    }

    function stringToArray(value) {
        return Uint8Array.from(value, c => c.charCodeAt(0));
    }

    function decodeBuffer(value) {
        return stringToArray(atob(value.replace(/-/g, '+').replace(/_/g, '/')));
    }

    function encodeBuffer(value) {
        try {
            return btoa(String.fromCharCode.apply(null, new Uint8Array(value)))
                .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
        } catch (error) {
            console.error('Error encoding buffer:', error);
            throw new Error('Failed to encode buffer.');
        }
    }

    async function register() {
        try {
            const response = await fetch('/webauthn/registration/begin', { method: 'POST' });
            if (!response.ok) {
                const msg = await response.text();
                throw new Error('Failed to get registration options from server: ' + msg);
            }
            const options = await response.json();
            options.publicKey.challenge = decodeBuffer(options.publicKey.challenge);
            options.publicKey.user.id = stringToArray(options.publicKey.user.id);

            const attestation = await navigator.credentials.create(options);

            const verificationResponse = await fetch('/webauthn/registration/finish', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    id: attestation.id,
                    rawId: encodeBuffer(attestation.rawId),
                    type: attestation.type,
                    response: {
                        attestationObject: encodeBuffer(attestation.response.attestationObject),
                        clientDataJSON: encodeBuffer(attestation.response.clientDataJSON),
                    }
                })
            });

            if (verificationResponse.ok) {
                window.location.reload();
            } else {
                const msg = await verificationResponse.text();
                throw new Error('Registration failed: ' + msg);
            }
        } catch (error) {
            console.error('Registration error:', error);
            alert("An error occurred during WebAuthn registration: " + error.message);
        }
    }

    async function login() {
        try {
            const response = await fetch('/webauthn/login/begin', { method: 'POST' });
            if (!response.ok) {
                const msg = await response.text();
                throw new Error('Failed to get login options from server: ' + msg);
            }
            const options = await response.json();
            options.publicKey.challenge = decodeBuffer(options.publicKey.challenge);

            if (options.publicKey.allowCredentials) {
                options.publicKey.allowCredentials.forEach(credential => {
                    credential.id = decodeBuffer(credential.id);
                });
            }

            const assertion = await navigator.credentials.get(options);

            const verificationResponse = await fetch('/webauthn/login/finish', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    id: assertion.id,
                    rawId: encodeBuffer(assertion.rawId),
                    type: assertion.type,
                    response: {
                        authenticatorData: encodeBuffer(assertion.response.authenticatorData),
                        clientDataJSON: encodeBuffer(assertion.response.clientDataJSON),
                        signature: encodeBuffer(assertion.response.signature),
                        userHandle: encodeBuffer(assertion.response.userHandle)
                    }
                })
            });

            if (verificationResponse.ok) {
                window.location.reload();
            } else {
                const msg = await verificationResponse.text();
                throw new Error('Login failed: ' + msg);
            }
        } catch (error) {
            console.error('Login error:', error);
            alert("An error occurred during WebAuthn login: " + error.message);
        }
    }

    const registerBtn = document.querySelector('#registerwebauthn');
    const loginBtn = document.querySelector('#loginwebauthn');

    if (registerBtn) {
        if (isWebAuthnSupported()) {
            registerBtn.classList.remove('hide');
            registerBtn.addEventListener('click', register);
        } else {
            console.warn('WebAuthn is not supported in this browser.');
        }
    }

    if (loginBtn) {
        if (isWebAuthnSupported()) {
            loginBtn.classList.remove('hide');
            loginBtn.addEventListener('click', login);
        } else {
            console.warn('WebAuthn is not supported in this browser.');
        }
    }

})();