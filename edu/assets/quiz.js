export function bindQuiz(form, answers) {
  form.addEventListener('submit', (event) => {
    event.preventDefault();
    const feedback = form.querySelector('.feedback');
    const selected = Object.fromEntries(new FormData(form));
    const correct = Object.entries(answers).every(([name, value]) => selected[name] === value);
    feedback.textContent = correct
      ? 'Correct. You have the governing rule: the required gate must report a result for every PR.'
      : 'Not quite. Revisit the amber warning, then try again from memory.';
    feedback.style.color = correct ? '#0b6e69' : '#9b4f00';
  });
}
