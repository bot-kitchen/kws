// Shared Recipe Action Configuration
// Used by both recipe-steps.js (form) and recipe view page (DAG)

// Action display names
const ACTION_DISPLAY_NAMES = {
    'add_liquid': 'Add Liquid',
    'add_solid': 'Add Solid',
    'heat': 'Heat',
    'agitate': 'Agitate/Mix',
    'pick_ingredient': 'Pick',
    'place_ingredient': 'Place',
    'open_pot_lid': 'Open Lid',
    'close_pot_lid': 'Close Lid',
    'acquire_pot_from_staging': 'Acquire Pot',
    'deliver_pot_to_serving': 'Deliver Pot'
};

// Action icons (Material Symbols)
const ACTION_ICONS = {
    'add_liquid': 'water_drop',
    'add_solid': 'add_circle',
    'heat': 'local_fire_department',
    'agitate': 'sync',
    'pick_ingredient': 'move_up',
    'place_ingredient': 'move_down',
    'open_pot_lid': 'expand_less',
    'close_pot_lid': 'expand_more',
    'acquire_pot_from_staging': 'move_up',
    'deliver_pot_to_serving': 'move_down'
};

// Action styling for DAG view (with light/dark theme support)
const ACTION_DAG_CONFIG = {
    'acquire_pot_from_staging': {
        bgClass: 'bg-gray-100 dark:bg-gray-500/20',
        borderClass: 'border-gray-300 dark:border-gray-500/50',
        textClass: 'text-gray-500 dark:text-gray-400'
    },
    'deliver_pot_to_serving': {
        bgClass: 'bg-gray-100 dark:bg-gray-500/20',
        borderClass: 'border-gray-300 dark:border-gray-500/50',
        textClass: 'text-gray-500 dark:text-gray-400'
    },
    'add_liquid': {
        bgClass: 'bg-blue-50 dark:bg-blue-500/20',
        borderClass: 'border-blue-200 dark:border-blue-500/50',
        textClass: 'text-blue-600 dark:text-blue-400'
    },
    'add_solid': {
        bgClass: 'bg-yellow-50 dark:bg-yellow-500/20',
        borderClass: 'border-yellow-300 dark:border-yellow-500/50',
        textClass: 'text-yellow-600 dark:text-yellow-400'
    },
    'pick_ingredient': {
        bgClass: 'bg-yellow-50 dark:bg-yellow-500/20',
        borderClass: 'border-yellow-300 dark:border-yellow-500/50',
        textClass: 'text-yellow-600 dark:text-yellow-400'
    },
    'place_ingredient': {
        bgClass: 'bg-yellow-50 dark:bg-yellow-500/20',
        borderClass: 'border-yellow-300 dark:border-yellow-500/50',
        textClass: 'text-yellow-600 dark:text-yellow-400'
    },
    'heat': {
        bgClass: 'bg-red-50 dark:bg-red-500/20',
        borderClass: 'border-red-200 dark:border-red-500/50',
        textClass: 'text-red-600 dark:text-red-400'
    },
    'agitate': {
        bgClass: 'bg-green-50 dark:bg-green-500/20',
        borderClass: 'border-green-200 dark:border-green-500/50',
        textClass: 'text-green-600 dark:text-green-400'
    },
    'open_pot_lid': {
        bgClass: 'bg-purple-50 dark:bg-purple-500/20',
        borderClass: 'border-purple-200 dark:border-purple-500/50',
        textClass: 'text-purple-600 dark:text-purple-400'
    },
    'close_pot_lid': {
        bgClass: 'bg-purple-50 dark:bg-purple-500/20',
        borderClass: 'border-purple-200 dark:border-purple-500/50',
        textClass: 'text-purple-600 dark:text-purple-400'
    }
};

// Default config for unknown actions
const DEFAULT_ACTION_CONFIG = {
    bgClass: 'bg-gray-100 dark:bg-gray-500/20',
    borderClass: 'border-gray-300 dark:border-gray-500/50',
    textClass: 'text-gray-500 dark:text-gray-400'
};

// Helper function to get action display name
function getActionDisplayName(action) {
    return ACTION_DISPLAY_NAMES[action] || action || 'Untitled';
}

// Helper function to get action icon
function getActionIcon(action) {
    return ACTION_ICONS[action] || 'help';
}

// Helper function to get full action config for DAG view
function getActionConfig(action) {
    const config = ACTION_DAG_CONFIG[action] || DEFAULT_ACTION_CONFIG;
    return {
        icon: getActionIcon(action),
        label: getActionDisplayName(action),
        ...config
    };
}

// Export for use in other scripts
if (typeof window !== 'undefined') {
    window.RecipeActions = {
        ACTION_DISPLAY_NAMES,
        ACTION_ICONS,
        ACTION_DAG_CONFIG,
        DEFAULT_ACTION_CONFIG,
        getActionDisplayName,
        getActionIcon,
        getActionConfig
    };
}
