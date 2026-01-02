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
        'subject-full-stop': [0], // Disable full-stop check (allow periods at end)
        'header-max-length': [0], // Disable header max length check
        'footer-max-line-length': [0], // Disable footer line length check
        'body-max-line-length': [0] // Disable body line length check
    }
};
