const acceptSelector = 'button[data-cookiebanner="accept_button"]'
const acceptButton = document.querySelector(acceptSelector)
if (acceptButton) {
    acceptButton.click()
    console.log("Accepting Cookies")
}
("");
