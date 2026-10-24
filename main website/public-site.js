// Navigation remains independent of graphics and account sessions.
const toggle = document.querySelector('.menu-toggle');
const navigation = document.querySelector('#site-navigation');
function closeNavigation(restoreFocus = false) {
  navigation.classList.remove('is-open');
  toggle.setAttribute('aria-expanded', 'false');
  if (restoreFocus) toggle.focus();
}
toggle.hidden = false;
document.documentElement.classList.add('navigation-ready');
toggle.addEventListener('click', () => {
  const open = toggle.getAttribute('aria-expanded') !== 'true';
  toggle.setAttribute('aria-expanded', String(open));
  navigation.classList.toggle('is-open', open);
});
navigation.addEventListener('click', event => {if (event.target.closest('a')) closeNavigation();});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && toggle.getAttribute('aria-expanded') === 'true') closeNavigation(true);
});
document.addEventListener('click', event => {if (!event.target.closest('.header')) closeNavigation();});
document.addEventListener('focusin', event => {if (!event.target.closest('.header')) closeNavigation();});
matchMedia('(max-width: 900px)').addEventListener('change', () => closeNavigation());
