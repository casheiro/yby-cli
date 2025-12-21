export default {
    extends: ['@commitlint/config-conventional'],
    rules: {
        'type-enum': [
            2,
            'always',
            [
                'build',
                'chore',
                'ci',
                'docs',
                'feat',
                'fix',
                'perf',
                'refactor',
                'revert',
                'style',
                'test',
                'release',
                'config', // Allow config updates
                'infra'   // Allow infrastructure changes
            ]
        ],
        'subject-case': [0], // Disable case check (allow Sentence case, etc)
        'subject-full-stop': [0] // Disable full-stop check (allow periods at end)
    }
};
