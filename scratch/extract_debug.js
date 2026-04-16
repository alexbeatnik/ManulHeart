const fs = require('fs');
const html = fs.readFileSync('scratch/test.html', 'utf8');

// The extract JS actually has `([target, hint]) => { ... }` so let's parse it and run it inside JSDOM if possible... wait, it lacks jsdom module globally. Let me just use go run to invoke CallProbe!
