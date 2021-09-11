const upcomingEventsTest = (more, pastEvents) => {
	if (!more) return false;
	if (!pastEvents) return true;
	if (more.getBoundingClientRect().y > pastEvents.getBoundingClientRect().y)
		return false;
	return true;
};

new Promise((resolve) => {
	const clickMore = () => {
		const more = document.querySelector('div[aria-label="See more"]');
		const pastEvents = [...document.querySelectorAll("span")].filter(
			(el) => el.innerText === "Past events",
		)[0];
		if (!upcomingEventsTest(more, pastEvents)) resolve();
		more.click();
		setTimeout(clickMore, 2000);
	};

	clickMore();
});
// so chromedp doesn't complain about undefined return value
("");
