/* Handles the "Notify me at launch" waitlist form on the Kalorie marketing page. */
(function () {
  var API_BASE_URL = 'https://kalorie.fit';
  var form = document.getElementById('remind-form');
  if (!form) return;

  var emailInput = document.getElementById('remind-email');
  var honeypot = form.querySelector('.remind-honeypot');
  var submitButton = document.getElementById('remind-submit');
  var status = document.getElementById('remind-status');

  function setStatus(message, state) {
    status.textContent = message;
    if (state) {
      status.setAttribute('data-state', state);
    } else {
      status.removeAttribute('data-state');
    }
  }

  form.addEventListener('submit', function (event) {
    event.preventDefault();

    if (honeypot && honeypot.value) {
      // Likely a bot filling every field; pretend success without hitting the API.
      setStatus("You're on the list — we'll email you at launch.", 'success');
      form.reset();
      return;
    }

    var email = emailInput.value.trim();
    if (!email) {
      setStatus('Enter an email address.', 'error');
      return;
    }

    submitButton.disabled = true;
    setStatus('Submitting...', null);

    fetch(API_BASE_URL + '/api/waitlist', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email }),
    })
      .then(function (response) {
        if (response.status === 409) {
          setStatus("You're already on the list — we'll email you at launch.", 'success');
          form.reset();
          return;
        }
        if (!response.ok) {
          throw new Error('Request failed');
        }
        setStatus("You're on the list — we'll email you at launch.", 'success');
        form.reset();
      })
      .catch(function () {
        setStatus('Something went wrong. Try again, or email support@kalorie.fit directly.', 'error');
      })
      .finally(function () {
        submitButton.disabled = false;
      });
  });
})();
