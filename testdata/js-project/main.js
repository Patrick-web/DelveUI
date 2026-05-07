/**
 * Entry point — call this for debugging.
 */
function fib(n) {
    let [a, b] = [0, 1];
    for (let i = 0; i < n; i++) {
        [a, b] = [b, a + b];
    }
    return a;
}

function main() {
    const name = "DelveUI";
    let count = 0;
    for (let i = 1; i <= 5; i++) {
        count += i;
        const msg = `Step ${i}: count=${count}, name=${name}`;
        console.log(msg);
    }
    const result = fib(10);
    console.log(`fib(10) = ${result}`);
    console.log(`Done. Total: ${count}`);
}

main();
