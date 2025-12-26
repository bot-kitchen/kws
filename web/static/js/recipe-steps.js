// Recipe Steps Management
// Global state
let steps = [];
let stepIdCounter = 1;
let ingredientGroupCounter = 1; // For tracking pick/add/place groups
// DAG visualization state (no external library needed)
let sortableInstance = null;
let tomSelectInstances = {}; // Store Tom Select instances for cleanup

// System step constants
const SYSTEM_STEP_ACQUIRE = 'acquire_pot_from_staging';
const SYSTEM_STEP_DELIVER = 'deliver_pot_to_serving';

// Check if a step is a system step (acquire pot or deliver pot)
function isSystemStep(step) {
    return step && step.isSystemStep === true;
}

// Create the acquire pot system step
function createAcquirePotStep(stepNumber) {
    return {
        id: stepIdCounter++,
        step_number: stepNumber,
        action: SYSTEM_STEP_ACQUIRE,
        parameters: {},
        depends_on_steps: [],
        isSystemStep: true
    };
}

// Create the deliver pot system step
function createDeliverPotStep(stepNumber) {
    return {
        id: stepIdCounter++,
        step_number: stepNumber,
        action: SYSTEM_STEP_DELIVER,
        parameters: {},
        depends_on_steps: [],
        isSystemStep: true
    };
}

// Ensure system steps exist and are in correct positions
function ensureSystemSteps() {
    // Check if acquire step exists
    let acquireStep = steps.find(s => s.action === SYSTEM_STEP_ACQUIRE);
    // Check if deliver step exists
    let deliverStep = steps.find(s => s.action === SYSTEM_STEP_DELIVER);

    if (!acquireStep) {
        // Add acquire step at the beginning
        acquireStep = createAcquirePotStep(1);
        steps.unshift(acquireStep);
    } else {
        // Mark existing acquire step as system step
        acquireStep.isSystemStep = true;
    }

    if (!deliverStep) {
        // Add deliver step at the end
        deliverStep = createDeliverPotStep(steps.length + 1);
        steps.push(deliverStep);
    } else {
        // Mark existing deliver step as system step
        deliverStep.isSystemStep = true;
    }

    // Ensure acquire is first and deliver is last
    enforceSystemStepPositions();
}

// Enforce that acquire pot is first and deliver pot is last
function enforceSystemStepPositions() {
    // Find acquire step and move to first position if not already
    const acquireIdx = steps.findIndex(s => s.action === SYSTEM_STEP_ACQUIRE);
    if (acquireIdx > 0) {
        const acquireStep = steps.splice(acquireIdx, 1)[0];
        steps.unshift(acquireStep);
    }

    // Find deliver step and move to last position if not already
    const deliverIdx = steps.findIndex(s => s.action === SYSTEM_STEP_DELIVER);
    if (deliverIdx >= 0 && deliverIdx < steps.length - 1) {
        const deliverStep = steps.splice(deliverIdx, 1)[0];
        steps.push(deliverStep);
    }

    // Renumber all steps to ensure correct numbering
    // (renumberSteps also updates deliver step's dependency)
    renumberSteps();
}

// Update deliver step to depend on the immediately preceding step
function updateDeliverStepDependency() {
    const deliverStep = steps.find(s => s.action === SYSTEM_STEP_DELIVER);
    if (deliverStep && steps.length > 1) {
        // The preceding step is at index (deliverStep's index - 1)
        const deliverIdx = steps.findIndex(s => s.action === SYSTEM_STEP_DELIVER);
        if (deliverIdx > 0) {
            const precedingStep = steps[deliverIdx - 1];
            deliverStep.depends_on_steps = [precedingStep.step_number];
        }
    }
}

// Get sorted ingredients (alphabetically by name)
function getSortedIngredients() {
    const ingredients = window.existingIngredients || [];
    return [...ingredients].sort((a, b) => a.name.localeCompare(b.name));
}

// Get sorted liquid ingredients
function getSortedLiquidIngredients() {
    return getSortedIngredients().filter(i => i.moisture_type === 'liquid');
}

// Get sorted solid ingredients
function getSortedSolidIngredients() {
    return getSortedIngredients().filter(i => i.moisture_type === 'dry' || i.moisture_type === 'wet');
}

// Initialize Tom Select on an ingredient select element (with search)
function initializeIngredientSelect(selectElement, stepId) {
    if (!selectElement || tomSelectInstances[stepId]) return;

    try {
        const tomSelect = new TomSelect(selectElement, {
            create: false,
            sortField: { field: 'text', direction: 'asc' },
            placeholder: 'Search ingredients...',
            controlInput: '<input type="text" autocomplete="off" size="1">',
            render: {
                option: function (data, escape) {
                    return `<div class="py-2 px-3">${escape(data.text)}</div>`;
                },
                item: function (data, escape) {
                    return `<div>${escape(data.text)}</div>`;
                },
                no_results: function () {
                    return '<div class="no-results py-2 px-3 text-gray-500">No ingredients found</div>';
                }
            }
        });
        tomSelectInstances[stepId] = tomSelect;
    } catch (error) {
        console.error('Error initializing Tom Select:', error);
    }
}

// Initialize Tom Select on a simple select element (no search needed)
function initializeSimpleSelect(selectElement, stepId) {
    if (!selectElement || tomSelectInstances[stepId]) return;

    try {
        const tomSelect = new TomSelect(selectElement, {
            create: false,
            controlInput: null, // No search input
            render: {
                option: function (data, escape) {
                    return `<div class="py-2 px-3">${escape(data.text)}</div>`;
                },
                item: function (data, escape) {
                    return `<div>${escape(data.text)}</div>`;
                }
            }
        });
        tomSelectInstances[stepId] = tomSelect;
    } catch (error) {
        console.error('Error initializing Tom Select:', error);
    }
}

// Destroy Tom Select instance for a step
function destroyIngredientSelect(stepId) {
    if (tomSelectInstances[stepId]) {
        try {
            tomSelectInstances[stepId].destroy();
        } catch (error) {
            console.error('Error destroying Tom Select instance:', error);
        }
        delete tomSelectInstances[stepId];
    }
}

// Destroy all Tom Select instances
function destroyAllSelectInstances() {
    Object.keys(tomSelectInstances).forEach(stepId => {
        destroyIngredientSelect(stepId);
    });
}

// Wait for DOM to be ready
document.addEventListener('DOMContentLoaded', function () {
    // Initialize if on recipe form page
    if (document.getElementById('add-step-btn')) {
        initializeRecipeStepsManagement();
    }
});

function initializeRecipeStepsManagement() {
    // Set up event listeners for Add Step buttons
    document.getElementById('add-step-btn').addEventListener('click', addStep);
    const bottomStepBtn = document.getElementById('add-step-btn-bottom');
    if (bottomStepBtn) {
        bottomStepBtn.addEventListener('click', addStep);
    }

    // Set up event listeners for Add Ingredient buttons
    const addIngredientBtn = document.getElementById('add-ingredient-btn');
    if (addIngredientBtn) {
        addIngredientBtn.addEventListener('click', addIngredientGroup);
    }
    const bottomIngredientBtn = document.getElementById('add-ingredient-btn-bottom');
    if (bottomIngredientBtn) {
        bottomIngredientBtn.addEventListener('click', addIngredientGroup);
    }

    // Override form submission to handle steps
    const form = document.getElementById('recipe-form');
    if (form) {
        form.addEventListener('submit', handleFormSubmit);
    }

    // Check for cloned recipe data (from clone button on recipe list)
    const clonedData = sessionStorage.getItem('clonedRecipe');
    if (clonedData) {
        try {
            const { recipe, steps } = JSON.parse(clonedData);
            sessionStorage.removeItem('clonedRecipe'); // Clear after reading

            // Pre-fill form fields
            document.getElementById('name').value = recipe.name + ' (Copy)';
            if (recipe.estimated_prep_time_sec) {
                document.getElementById('estimated_prep_time_sec').value = Math.round(recipe.estimated_prep_time_sec / 60);
            }
            if (recipe.estimated_cooking_time_sec) {
                document.getElementById('estimated_cooking_time_sec').value = Math.round(recipe.estimated_cooking_time_sec / 60);
            }

            // Initialize steps from cloned data
            if (steps && steps.length > 0) {
                initializeSteps(steps);
            }
            ensureSystemSteps();
            renderSteps();
            updateDAG();
            return; // Skip loading existing recipe steps (we're creating new)
        } catch (e) {
            console.error('Error loading cloned recipe:', e);
            sessionStorage.removeItem('clonedRecipe');
        }
    }

    // Initialize from existing data if available (editing mode)
    if (window.existingRecipeSteps && window.existingRecipeSteps.length > 0) {
        initializeSteps(window.existingRecipeSteps);
    }

    // Ensure system steps exist (acquire pot first, deliver pot last)
    ensureSystemSteps();
    renderSteps();
    updateDAG();
}

// Initialize from existing steps (when editing)
function initializeSteps(existingSteps) {
    try {
        // Sort by step_number first
        const sortedSteps = [...existingSteps].sort((a, b) => a.step_number - b.step_number);

        // Detect ingredient groups (pick followed by add_solid followed by place)
        const processedIndices = new Set();

        steps = [];
        for (let i = 0; i < sortedSteps.length; i++) {
            if (processedIndices.has(i)) continue;

            const step = sortedSteps[i];
            const params = typeof step.parameters === 'string' ? JSON.parse(step.parameters || '{}') : (step.parameters || {});
            const deps = typeof step.depends_on_steps === 'string' ? JSON.parse(step.depends_on_steps || '[]') : (step.depends_on_steps || []);

            // Check if this is a pick_ingredient that starts a group
            // Group detection is based on sequence pattern (pick → add_solid → place)
            // rather than strict ingredient ID matching to handle legacy data
            if (step.action === 'pick_ingredient') {
                // Look for add_solid and place_ingredient in the next 2 steps
                let addSolidIdx = -1;
                let placeIdx = -1;

                for (let j = i + 1; j < sortedSteps.length && j <= i + 2; j++) {
                    const nextStep = sortedSteps[j];

                    if (nextStep.action === 'add_solid' && addSolidIdx === -1) {
                        addSolidIdx = j;
                    } else if (nextStep.action === 'place_ingredient' && placeIdx === -1) {
                        placeIdx = j;
                    }
                }

                // If we found a complete group, mark them all with the same groupId
                if (addSolidIdx !== -1 && placeIdx !== -1) {
                    const groupId = ingredientGroupCounter++;

                    // Add pick step
                    steps.push({
                        id: stepIdCounter++,
                        step_number: step.step_number,
                        action: step.action,
                        parameters: params,
                        depends_on_steps: deps,
                        ingredientGroupId: groupId,
                        ingredientGroupRole: 'pick'
                    });
                    processedIndices.add(i);

                    // Add add_solid step
                    const addStep = sortedSteps[addSolidIdx];
                    const addParams = typeof addStep.parameters === 'string' ? JSON.parse(addStep.parameters || '{}') : (addStep.parameters || {});
                    const addDeps = typeof addStep.depends_on_steps === 'string' ? JSON.parse(addStep.depends_on_steps || '[]') : (addStep.depends_on_steps || []);
                    steps.push({
                        id: stepIdCounter++,
                        step_number: addStep.step_number,
                        action: addStep.action,
                        parameters: addParams,
                        depends_on_steps: addDeps,
                        ingredientGroupId: groupId,
                        ingredientGroupRole: 'add'
                    });
                    processedIndices.add(addSolidIdx);

                    // Add place step
                    const placeStep = sortedSteps[placeIdx];
                    const placeParams = typeof placeStep.parameters === 'string' ? JSON.parse(placeStep.parameters || '{}') : (placeStep.parameters || {});
                    const placeDeps = typeof placeStep.depends_on_steps === 'string' ? JSON.parse(placeStep.depends_on_steps || '[]') : (placeStep.depends_on_steps || []);
                    steps.push({
                        id: stepIdCounter++,
                        step_number: placeStep.step_number,
                        action: placeStep.action,
                        parameters: placeParams,
                        depends_on_steps: placeDeps,
                        ingredientGroupId: groupId,
                        ingredientGroupRole: 'place'
                    });
                    processedIndices.add(placeIdx);

                    continue;
                }
            }

            // Regular step (not part of a group)
            steps.push({
                id: stepIdCounter++,
                step_number: step.step_number,
                action: step.action || '',
                parameters: params,
                depends_on_steps: deps
            });
            processedIndices.add(i);
        }

        renderSteps();
        updateDAG();
    } catch (error) {
        console.error('Error initializing steps:', error);
    }
}

// Add new step (inserts before the deliver step)
function addStep() {
    const step = {
        id: stepIdCounter++,
        step_number: steps.length, // Will be renumbered
        action: '',
        parameters: {},
        depends_on_steps: []
    };

    // Insert before the last step (deliver pot) if it exists
    const deliverIndex = steps.findIndex(s => s.action === SYSTEM_STEP_DELIVER);
    if (deliverIndex > 0) {
        steps.splice(deliverIndex, 0, step);
    } else {
        steps.push(step);
    }

    renumberSteps();
    renderSteps();
    updateDAG();
}

// Add ingredient group (pick + add_solid + place as atomic unit)
// Inserts before the deliver step
function addIngredientGroup() {
    const groupId = ingredientGroupCounter++;

    // Create pick_ingredient step
    const pickStep = {
        id: stepIdCounter++,
        step_number: 0, // Will be renumbered
        action: 'pick_ingredient',
        parameters: {},
        depends_on_steps: [],
        ingredientGroupId: groupId,
        ingredientGroupRole: 'pick'
    };

    // Create add_solid step (depends on pick)
    const addSolidStep = {
        id: stepIdCounter++,
        step_number: 0, // Will be renumbered
        action: 'add_solid',
        parameters: {},
        depends_on_steps: [], // Will be set after renumbering
        ingredientGroupId: groupId,
        ingredientGroupRole: 'add'
    };

    // Create place_ingredient step (depends on add)
    const placeStep = {
        id: stepIdCounter++,
        step_number: 0, // Will be renumbered
        action: 'place_ingredient',
        parameters: {},
        depends_on_steps: [], // Will be set after renumbering
        ingredientGroupId: groupId,
        ingredientGroupRole: 'place'
    };

    // Insert before the last step (deliver pot) if it exists
    const deliverIndex = steps.findIndex(s => s.action === SYSTEM_STEP_DELIVER);
    if (deliverIndex > 0) {
        steps.splice(deliverIndex, 0, pickStep, addSolidStep, placeStep);
    } else {
        steps.push(pickStep, addSolidStep, placeStep);
    }

    renumberSteps();

    // Update intra-group dependencies after renumbering
    const pickStepNum = pickStep.step_number;
    addSolidStep.depends_on_steps = [pickStepNum];
    placeStep.depends_on_steps = [pickStepNum + 1];

    renderSteps();
    updateDAG();
}

// Update ingredient across all steps in a group
function updateIngredientGroupIngredient(groupId, ingredientId, quantity) {
    const groupSteps = steps.filter(s => s.ingredientGroupId === groupId);
    const ingredient = (window.existingIngredients || []).find(i => i.id === ingredientId);
    groupSteps.forEach(step => {
        step.parameters.ingredient_id = ingredientId;
        if (ingredient) {
            step.parameters.ingredient_name = ingredient.name;
        }
        if (step.ingredientGroupRole === 'add' && quantity !== undefined) {
            step.parameters.quantity = quantity;
        }
    });
    renderSteps();
    updateDAG();
}

// Update dependencies for any grouped step (preserving mandatory intra-group dependency)
function updateGroupedStepDependencies(step) {
    if (!step || !step.ingredientGroupId) return;

    const card = document.querySelector(`[data-step-id="${step.id}"]`);
    if (!card) return;

    // Find the mandatory dependency based on role:
    // - pick: no mandatory dependency
    // - add: pick step is mandatory
    // - place: add step is mandatory
    let mandatoryDepStepNumber = null;
    if (step.ingredientGroupRole === 'add') {
        const pickStep = steps.find(s => s.ingredientGroupId === step.ingredientGroupId && s.ingredientGroupRole === 'pick');
        mandatoryDepStepNumber = pickStep ? pickStep.step_number : null;
    } else if (step.ingredientGroupRole === 'place') {
        const addStep = steps.find(s => s.ingredientGroupId === step.ingredientGroupId && s.ingredientGroupRole === 'add');
        mandatoryDepStepNumber = addStep ? addStep.step_number : null;
    }

    // Get checked additional dependencies from checkboxes
    const checkboxes = card.querySelectorAll('.dependency-checkbox:checked');
    const additionalDeps = Array.from(checkboxes).map(cb => parseInt(cb.value));

    // Rebuild dependencies: mandatory dependency (if any) + additional selected steps
    step.depends_on_steps = mandatoryDepStepNumber ? [mandatoryDepStepNumber, ...additionalDeps] : additionalDeps;

    // Update hidden input
    const hiddenInput = card.querySelector('.dependencies-select');
    if (hiddenInput) {
        hiddenInput.value = step.depends_on_steps.join(',');
    }

    updateDAG();
}

// Delete step (handles group deletion)
function deleteStep(stepId) {
    const stepToDelete = steps.find(s => s.id === stepId);
    if (!stepToDelete) return;

    // Prevent deletion of system steps (acquire pot, deliver pot)
    if (isSystemStep(stepToDelete)) {
        alert('System steps (Acquire Pot and Deliver Pot) cannot be deleted.');
        return;
    }

    let stepsToDelete = [stepId];
    let deletedStepNumbers = [stepToDelete.step_number];

    // If this step is part of an ingredient group, delete all steps in the group
    if (stepToDelete.ingredientGroupId) {
        const groupSteps = steps.filter(s => s.ingredientGroupId === stepToDelete.ingredientGroupId);
        stepsToDelete = groupSteps.map(s => s.id);
        deletedStepNumbers = groupSteps.map(s => s.step_number);
    }

    // Destroy Tom Select instances for deleted steps
    stepsToDelete.forEach(id => {
        destroyIngredientSelect(`group-${id}`);
        destroyIngredientSelect(`liquid-${id}`);
    });

    // Remove the steps
    steps = steps.filter(s => !stepsToDelete.includes(s.id));

    // Update dependencies that referenced the deleted steps
    deletedStepNumbers.forEach(deletedNum => {
        steps.forEach(step => {
            step.depends_on_steps = step.depends_on_steps.filter(num => num !== deletedNum);
        });
    });

    renumberSteps();
    renderSteps();
    updateDAG();
}

// Renumber steps after deletion/reordering
function renumberSteps() {
    // Build mapping of old to new step numbers
    const oldToNew = {};
    steps.forEach((step, index) => {
        oldToNew[step.step_number] = index + 1;
    });

    // Update step numbers
    steps.forEach((step, index) => {
        step.step_number = index + 1;
    });

    // Update all dependency references to use new step numbers
    steps.forEach(step => {
        step.depends_on_steps = step.depends_on_steps
            .map(oldNum => oldToNew[oldNum])
            .filter(num => num !== undefined && num < step.step_number);
    });

    // Always update deliver step's dependency on the preceding step
    updateDeliverStepDependency();
}

// Update step from form fields
function updateStepFromCard(stepId) {
    const step = steps.find(s => s.id === stepId);
    if (!step) return;

    // System steps don't have editable form fields - skip them
    // Their dependencies are managed by updateDeliverStepDependency()
    if (isSystemStep(step)) {
        return;
    }

    const card = document.querySelector(`[data-step-id="${stepId}"]`);
    if (!card) return;

    // For grouped steps, use the specialized update function
    if (step.ingredientGroupId) {
        updateGroupedStepFromCard(step, card);
        return;
    }

    // Get action from radio buttons (only for non-grouped steps)
    const actionRadio = card.querySelector('.action-radio:checked');
    if (actionRadio) {
        step.action = actionRadio.value;
    }

    // Get dependencies from checkboxes
    const checkboxes = card.querySelectorAll('.dependency-checkbox:checked');
    step.depends_on_steps = Array.from(checkboxes).map(cb => parseInt(cb.value));

    // Update hidden input for form data
    const hiddenInput = card.querySelector('.dependencies-select');
    if (hiddenInput) {
        hiddenInput.value = step.depends_on_steps.join(',');
    }

    // Build parameters based on action
    step.parameters = buildParameters(step.action, card);

    updateDAG();
}

// Update grouped step from card (pick, add, place)
function updateGroupedStepFromCard(step, card) {
    // Update dependencies using the grouped step handler
    updateGroupedStepDependencies(step);

    // Role-specific updates
    if (step.ingredientGroupRole === 'pick') {
        // Ingredient selector is on pick step
        const ingredientSelect = card.querySelector('.ingredient-select-group');
        if (ingredientSelect && ingredientSelect.value) {
            const ingredientId = parseInt(ingredientSelect.value);
            const ingredient = (window.existingIngredients || []).find(i => i.id === ingredientId);
            step.parameters.ingredient_id = ingredientId;
            if (ingredient) {
                step.parameters.ingredient_name = ingredient.name;
            }
        }
    } else if (step.ingredientGroupRole === 'add') {
        // Quantity input is on add step
        const quantityInput = card.querySelector('.quantity-input');
        if (quantityInput) {
            step.parameters.quantity = parseFloat(quantityInput.value) || 0;
            step.parameters.metric = 'grams';
        }
    }
    // place step doesn't have additional inputs beyond dependencies
}

// Build parameters JSON based on action and form inputs
function buildParameters(action, card) {
    switch (action) {
        case 'add_liquid': {
            const selectEl = card.querySelector('.ingredient-select-liquid');
            const ingredientId = parseInt(selectEl?.value || 0);
            const ingredientName = selectEl?.selectedOptions[0]?.text || '';
            const params = {
                quantity: parseFloat(card.querySelector('.param-add-liquid .quantity-input')?.value || 0),
                metric: 'ml'
            };
            if (ingredientId > 0) {
                params.ingredient_id = ingredientId;
                params.ingredient_name = ingredientName;
            }
            return params;
        }
        case 'add_solid': {
            const selectEl = card.querySelector('.ingredient-select-solid');
            const ingredientId = parseInt(selectEl?.value || 0);
            const ingredientName = selectEl?.selectedOptions[0]?.text || '';
            const params = {
                quantity: parseFloat(card.querySelector('.param-add-solid .quantity-input')?.value || 0),
                metric: 'grams'
            };
            if (ingredientId > 0) {
                params.ingredient_id = ingredientId;
                params.ingredient_name = ingredientName;
            }
            return params;
        }
        case 'heat':
            return {
                power_level: parseInt(card.querySelector('.power-level-input')?.value || 3),
                on_duration_sec: parseInt(card.querySelector('.heat-duration-input')?.value || 0)
            };
        case 'agitate':
            return {
                speed: card.querySelector('.speed-radio:checked')?.value || 'slow_stir',
                duration_sec: parseInt(card.querySelector('.agitate-duration-input')?.value || 30),
                direction: card.querySelector('.direction-radio:checked')?.value || 'scraping'
            };
        case 'pick_ingredient': {
            const ingredientId = parseInt(card.querySelector('.ingredient-select-pick')?.value || 0);
            const params = {};
            if (ingredientId > 0) params.ingredient_id = ingredientId;
            return params;
        }
        case 'place_ingredient': {
            const ingredientId = parseInt(card.querySelector('.ingredient-select-place')?.value || 0);
            const params = {};
            if (ingredientId > 0) params.ingredient_id = ingredientId;
            return params;
        }
        default:
            return {};
    }
}


// Render all steps
function renderSteps() {
    const container = document.getElementById('steps-container');

    // Destroy all existing Tom Select instances before re-rendering
    destroyAllSelectInstances();

    if (steps.length === 0) {
        container.innerHTML = `
            <div id="no-steps-message" class="text-center py-8 text-gray-500 dark:text-text-secondary">
                <span class="material-symbols-outlined text-4xl mb-2 block text-gray-400">format_list_numbered</span>
                <p>No steps added yet. Click "Add Step" to begin.</p>
            </div>
        `;
        return;
    }

    container.innerHTML = '';

    steps.forEach((step, index) => {
        const card = createStepCard(step, index);
        container.appendChild(card);
    });

    // Initialize Tom Select on all selects after cards are in DOM
    steps.forEach((step) => {
        const card = document.querySelector(`[data-step-id="${step.id}"]`);
        if (!card) return;

        // Initialize Tom Select for grouped ingredient selects (pick step)
        const groupSelect = card.querySelector('.ingredient-select-group');
        if (groupSelect) {
            initializeIngredientSelect(groupSelect, `group-${step.id}`);
        }

        // Initialize Tom Select for regular step ingredient selects
        const liquidSelect = card.querySelector('.ingredient-select-liquid');
        if (liquidSelect) {
            initializeIngredientSelect(liquidSelect, `liquid-${step.id}`);
        }

        // Note: Action, Speed, and Direction now use radio buttons, no Tom Select needed
    });

    // Initialize sortable for drag-and-drop
    initializeSortable();
}

// Create a step card element
function createStepCard(step, index) {
    // Ensure step.parameters is always an object to prevent errors
    if (!step.parameters || typeof step.parameters !== 'object') {
        step.parameters = {};
    }

    const card = document.createElement('div');
    const isGrouped = !!step.ingredientGroupId;
    const isFirstInGroup = step.ingredientGroupRole === 'pick';
    const isLastInGroup = step.ingredientGroupRole === 'place';
    const isSystem = isSystemStep(step);

    // Styling for grouped steps and system steps
    let cardClass = 'step-card border border-gray-200 dark:border-border-dark rounded-lg p-4 bg-white dark:bg-surface-dark';
    if (isSystem) {
        // Special styling for system steps - subtle muted background
        cardClass = 'step-card border border-gray-100 dark:border-border-dark rounded-lg p-4 bg-gray-50 dark:bg-surface-dark';
    } else if (isGrouped) {
        cardClass = 'step-card border-l-4 border-l-primary border border-gray-200 dark:border-border-dark p-4 bg-white dark:bg-surface-dark';
        if (isFirstInGroup) {
            cardClass += ' rounded-t-lg rounded-b-none border-b-0';
        } else if (isLastInGroup) {
            cardClass += ' rounded-b-lg rounded-t-none';
        } else {
            cardClass += ' rounded-none border-b-0';
        }
    }
    card.className = cardClass;
    card.setAttribute('data-step-id', step.id);
    card.setAttribute('data-step-number', step.step_number);
    if (isGrouped) {
        card.setAttribute('data-ingredient-group-id', step.ingredientGroupId);
    }
    if (isSystem) {
        card.setAttribute('data-system-step', 'true');
    }

    // Get sorted ingredients (alphabetically)
    const liquidIngredients = getSortedLiquidIngredients();
    const solidIngredients = getSortedSolidIngredients();

    // For system steps, show read-only card with lock icon
    if (isSystem) {
        const isAcquire = step.action === SYSTEM_STEP_ACQUIRE;
        const systemLabel = isAcquire ? 'Acquire Pot from Staging' : 'Deliver Pot to Serving';
        const systemIcon = isAcquire ? 'move_up' : 'move_down';
        const systemDescription = isAcquire
            ? 'This step acquires a pot from staging to begin cooking.'
            : 'This step delivers the finished pot to the serving area.';

        card.innerHTML = `
            <div class="flex items-start gap-4">
                <!-- Step Number Badge -->
                <div class="flex-shrink-0">
                    <div class="w-10 h-10 rounded-full bg-gray-100 dark:bg-surface-highlight flex items-center justify-center">
                        <span class="step-number-display font-bold text-gray-500 dark:text-text-secondary text-lg">${step.step_number}</span>
                    </div>
                </div>

                <!-- Step Content -->
                <div class="flex-1 space-y-2">
                    <div class="flex items-center gap-2">
                        <span class="material-symbols-outlined text-gray-400 dark:text-text-secondary">${systemIcon}</span>
                        <span class="text-sm font-medium text-gray-600 dark:text-gray-300">${systemLabel}</span>
                        <span class="text-xs px-2 py-0.5 rounded-full bg-gray-100 dark:bg-surface-highlight text-gray-500 dark:text-text-secondary font-medium flex items-center gap-1">
                            <span class="material-symbols-outlined text-xs">lock</span>
                            System
                        </span>
                    </div>
                    <p class="text-xs text-gray-400 dark:text-text-secondary">${systemDescription}</p>
                </div>

                <!-- Lock Icon (no delete button) -->
                <div class="flex-shrink-0 p-2 text-gray-300 dark:text-text-secondary" title="System step cannot be deleted">
                    <span class="material-symbols-outlined">lock</span>
                </div>
            </div>
        `;

        return card;
    }

    // For grouped steps, show simplified UI
    if (isGrouped) {
        const roleLabels = {
            'pick': 'Pick Ingredient',
            'add': 'Add Solid',
            'place': 'Place Ingredient'
        };
        const roleIcons = {
            'pick': 'move_up',
            'add': 'add_circle',
            'place': 'move_down'
        };

        card.innerHTML = `
            <div class="flex items-start gap-4">
                <!-- Step Number Badge -->
                <div class="flex-shrink-0">
                    <div class="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center">
                        <span class="step-number-display font-bold text-primary text-lg">${step.step_number}</span>
                    </div>
                </div>

                <!-- Step Content -->
                <div class="flex-1 space-y-3">
                    <div class="flex items-center gap-2">
                        <span class="material-symbols-outlined text-primary">${roleIcons[step.ingredientGroupRole]}</span>
                        <span class="text-sm font-medium text-gray-900 dark:text-white">${roleLabels[step.ingredientGroupRole]}</span>
                        <span class="text-xs px-2 py-0.5 rounded-full bg-primary/10 text-primary font-medium">Grouped</span>
                    </div>

                    ${step.ingredientGroupRole === 'pick' ? `
                        <!-- Ingredient selector (only on pick step, syncs to others) -->
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                Ingredient <span class="text-red-500">*</span>
                            </label>
                            <select class="ingredient-select-group w-full">
                                <option value="">Select ingredient...</option>
                                ${solidIngredients.map(ing => `<option value="${ing.id}" ${parseInt(step.parameters.ingredient_id) === ing.id ? 'selected' : ''}>${ing.name}</option>`).join('')}
                            </select>
                        </div>
                        <!-- Dependencies for pick step -->
                        <div class="mt-3">
                            <label class="flex items-center gap-2 mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                                <span class="material-symbols-outlined text-sm text-gray-500">account_tree</span>
                                Dependencies
                            </label>
                            <p class="text-xs text-gray-500 dark:text-text-secondary mb-2">
                                Steps that must complete before picking this ingredient
                            </p>
                            ${getAvailableDependenciesForGroupedStep(step, index)}
                            <input type="hidden" class="dependencies-select" value="${step.depends_on_steps.join(',')}">
                        </div>
                    ` : step.ingredientGroupRole === 'add' ? `
                        <!-- Quantity input (only on add step) -->
                        <div class="flex items-center gap-2 text-sm text-gray-600 dark:text-text-secondary">
                            <span class="material-symbols-outlined text-base">inventory_2</span>
                            <span class="ingredient-name-display">${getIngredientName(step.parameters.ingredient_id) || 'No ingredient selected'}</span>
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                                Quantity (grams) <span class="text-red-500">*</span>
                            </label>
                            <input type="number" class="quantity-input w-full px-4 py-2.5 bg-gray-50 dark:bg-surface-highlight border border-gray-300 dark:border-border-dark rounded-lg"
                                   min="0.1" step="0.1" value="${step.parameters.quantity || ''}">
                        </div>
                        <!-- Additional Dependencies for add_solid step -->
                        <div class="mt-3">
                            <label class="flex items-center gap-2 mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                                <span class="material-symbols-outlined text-sm text-gray-500">account_tree</span>
                                Additional Dependencies
                            </label>
                            <p class="text-xs text-gray-500 dark:text-text-secondary mb-2">
                                Add steps that must complete before adding this ingredient (e.g., heat)
                            </p>
                            ${getAvailableDependenciesForGroupedStep(step, index)}
                            <input type="hidden" class="dependencies-select" value="${step.depends_on_steps.join(',')}">
                        </div>
                    ` : `
                        <!-- Ingredient name and dependencies for place step -->
                        <div class="flex items-center gap-2 text-sm text-gray-600 dark:text-text-secondary">
                            <span class="material-symbols-outlined text-base">inventory_2</span>
                            <span class="ingredient-name-display">${getIngredientName(step.parameters.ingredient_id) || 'No ingredient selected'}</span>
                        </div>
                        <!-- Dependencies for place step -->
                        <div class="mt-3">
                            <label class="flex items-center gap-2 mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                                <span class="material-symbols-outlined text-sm text-gray-500">account_tree</span>
                                Additional Dependencies
                            </label>
                            <p class="text-xs text-gray-500 dark:text-text-secondary mb-2">
                                Add steps that must complete before placing this ingredient
                            </p>
                            ${getAvailableDependenciesForGroupedStep(step, index)}
                            <input type="hidden" class="dependencies-select" value="${step.depends_on_steps.join(',')}">
                        </div>
                    `}
                </div>

                <!-- Delete Button (only on first step of group) -->
                ${isFirstInGroup ? `
                    <button type="button" class="delete-step-btn flex-shrink-0 p-2 rounded-lg text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors" title="Delete ingredient group">
                        <span class="material-symbols-outlined">delete</span>
                    </button>
                ` : ''}
            </div>
        `;
    } else {
        // Regular (non-grouped) step card
        card.innerHTML = `
            <div class="flex items-start gap-4">
                <!-- Drag Handle -->
                <div class="drag-handle cursor-move pt-2">
                    <span class="material-symbols-outlined text-gray-400">drag_indicator</span>
                </div>

                <!-- Step Number Badge -->
                <div class="flex-shrink-0">
                    <div class="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center">
                        <span class="step-number-display font-bold text-primary text-lg">${step.step_number}</span>
                    </div>
                </div>

                <!-- Step Content -->
                <div class="flex-1 space-y-4">
                    <!-- Action Selector -->
                    <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                            Action <span class="text-red-500">*</span>
                        </label>
                        <div class="inline-flex rounded-lg border border-gray-300 dark:border-border-dark overflow-hidden action-group">
                            <label class="cursor-pointer action-btn ${step.action === 'add_liquid' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="action-${step.id}" value="add_liquid" class="sr-only action-radio" ${step.action === 'add_liquid' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Add Liquid
                                </div>
                            </label>
                            <label class="cursor-pointer action-btn border-l border-gray-300 dark:border-border-dark ${step.action === 'heat' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="action-${step.id}" value="heat" class="sr-only action-radio" ${step.action === 'heat' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Heat
                                </div>
                            </label>
                            <label class="cursor-pointer action-btn border-l border-gray-300 dark:border-border-dark ${step.action === 'agitate' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="action-${step.id}" value="agitate" class="sr-only action-radio" ${step.action === 'agitate' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Agitate/Mix
                                </div>
                            </label>
                            <label class="cursor-pointer action-btn border-l border-gray-300 dark:border-border-dark ${step.action === 'open_pot_lid' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="action-${step.id}" value="open_pot_lid" class="sr-only action-radio" ${step.action === 'open_pot_lid' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Open Lid
                                </div>
                            </label>
                            <label class="cursor-pointer action-btn border-l border-gray-300 dark:border-border-dark ${step.action === 'close_pot_lid' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="action-${step.id}" value="close_pot_lid" class="sr-only action-radio" ${step.action === 'close_pot_lid' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Close Lid
                                </div>
                            </label>
                        </div>
                    </div>

                    <!-- add_liquid parameters -->
                    <div class="param-add-liquid ${step.action === 'add_liquid' ? '' : 'hidden'}">
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                            Ingredient <span class="text-red-500">*</span>
                        </label>
                        <select class="ingredient-select-liquid w-full mb-4">
                            <option value="">Select ingredient...</option>
                            ${liquidIngredients.map(ing => `<option value="${ing.id}" ${parseInt(step.parameters.ingredient_id) === ing.id ? 'selected' : ''}>${ing.name}</option>`).join('')}
                        </select>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                            Quantity (ml) <span class="text-red-500">*</span>
                        </label>
                        <input type="number" class="quantity-input w-full px-4 py-2.5 bg-gray-50 dark:bg-surface-highlight border border-gray-300 dark:border-border-dark rounded-lg"
                               min="0.1" step="0.1" value="${step.parameters.quantity || ''}">
                    </div>

                    <!-- heat parameters -->
                    <div class="param-heat ${step.action === 'heat' ? '' : 'hidden'}">
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                            Power Level (1-10) <span class="text-red-500">*</span>
                        </label>
                        <input type="number" class="power-level-input w-full px-4 py-2.5 bg-gray-50 dark:bg-surface-highlight border border-gray-300 dark:border-border-dark rounded-lg mb-4"
                               min="1" max="10" step="1" value="${step.parameters.power_level || 3}">
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                            Duration (seconds) <span class="text-red-500">*</span>
                        </label>
                        <input type="number" class="heat-duration-input w-full px-4 py-2.5 bg-gray-50 dark:bg-surface-highlight border border-gray-300 dark:border-border-dark rounded-lg"
                               min="1" step="1" value="${step.parameters.on_duration_sec || ''}">
                    </div>

                    <!-- agitate parameters -->
                    <div class="param-agitate ${step.action === 'agitate' ? '' : 'hidden'}">
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Speed</label>
                        <div class="inline-flex rounded-lg border border-gray-300 dark:border-border-dark overflow-hidden mb-4 speed-group">
                            <label class="cursor-pointer speed-btn ${!step.parameters.speed || step.parameters.speed === 'slow_stir' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="speed-${step.id}" value="slow_stir" class="sr-only speed-radio" ${!step.parameters.speed || step.parameters.speed === 'slow_stir' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Slow Stir
                                </div>
                            </label>
                            <label class="cursor-pointer speed-btn border-l border-gray-300 dark:border-border-dark ${step.parameters.speed === 'med_stir' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="speed-${step.id}" value="med_stir" class="sr-only speed-radio" ${step.parameters.speed === 'med_stir' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Medium Stir
                                </div>
                            </label>
                            <label class="cursor-pointer speed-btn border-l border-gray-300 dark:border-border-dark ${step.parameters.speed === 'fast_stir' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="speed-${step.id}" value="fast_stir" class="sr-only speed-radio" ${step.parameters.speed === 'fast_stir' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Fast Stir
                                </div>
                            </label>
                            <label class="cursor-pointer speed-btn border-l border-gray-300 dark:border-border-dark ${step.parameters.speed === 'coarse_grind' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="speed-${step.id}" value="coarse_grind" class="sr-only speed-radio" ${step.parameters.speed === 'coarse_grind' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Coarse Grind
                                </div>
                            </label>
                            <label class="cursor-pointer speed-btn border-l border-gray-300 dark:border-border-dark ${step.parameters.speed === 'fine_grind' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="speed-${step.id}" value="fine_grind" class="sr-only speed-radio" ${step.parameters.speed === 'fine_grind' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Fine Grind
                                </div>
                            </label>
                        </div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Duration (seconds) <span class="text-red-500">*</span></label>
                        <input type="number" class="agitate-duration-input w-full px-4 py-2.5 bg-gray-50 dark:bg-surface-highlight border border-gray-300 dark:border-border-dark rounded-lg mb-4"
                               min="1" step="1" value="${step.parameters.duration_sec || ''}">
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Direction</label>
                        <div class="inline-flex rounded-lg border border-gray-300 dark:border-border-dark overflow-hidden direction-group">
                            <label class="cursor-pointer direction-btn ${!step.parameters.direction || step.parameters.direction === 'scraping' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="direction-${step.id}" value="scraping" class="sr-only direction-radio" ${!step.parameters.direction || step.parameters.direction === 'scraping' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Scraping
                                </div>
                            </label>
                            <label class="cursor-pointer direction-btn border-l border-gray-300 dark:border-border-dark ${step.parameters.direction === 'cutting' ? 'bg-primary text-white' : 'bg-white dark:bg-surface-dark text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-highlight'}">
                                <input type="radio" name="direction-${step.id}" value="cutting" class="sr-only direction-radio" ${step.parameters.direction === 'cutting' ? 'checked' : ''}>
                                <div class="px-3 py-1.5 text-center text-sm font-medium transition-colors">
                                    Cutting
                                </div>
                            </label>
                        </div>
                    </div>

                    <!-- Dependencies -->
                    <div>
                        <label class="flex items-center gap-2 mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                            <span class="material-symbols-outlined text-sm text-gray-500">account_tree</span>
                            Dependencies (optional)
                        </label>
                        <p class="text-xs text-gray-500 dark:text-text-secondary mb-2">
                            Select which steps must complete before this step can begin
                        </p>
                        ${index === 0 ? `
                            <p class="text-xs text-gray-400 dark:text-text-secondary italic py-2">No previous steps available</p>
                        ` : `
                            <div class="dependencies-container space-y-2 max-h-48 overflow-y-auto p-3 bg-gray-50 dark:bg-surface-highlight border border-gray-300 dark:border-border-dark rounded-lg">
                                ${steps.slice(0, index).reverse().slice(0, 4).map(s => `
                                    <label class="flex items-center gap-3 p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-border-dark cursor-pointer transition-colors group">
                                        <input type="checkbox" class="dependency-checkbox w-4 h-4 rounded border-gray-300 dark:border-border-dark text-primary focus:ring-primary focus:ring-offset-0 dark:bg-surface-dark"
                                               value="${s.step_number}" ${step.depends_on_steps.includes(s.step_number) ? 'checked' : ''}>
                                        <div class="flex items-center gap-2 flex-1 min-w-0">
                                            <span class="flex-shrink-0 w-6 h-6 rounded-full bg-primary/10 flex items-center justify-center text-xs font-bold text-primary">${s.step_number}</span>
                                            <span class="text-sm text-gray-700 dark:text-gray-300 truncate">${getActionDisplayName(s.action) || 'Untitled'}</span>
                                        </div>
                                    </label>
                                `).join('')}
                            </div>
                        `}
                        <input type="hidden" class="dependencies-select" value="${step.depends_on_steps.join(',')}">
                    </div>
                </div>

                <!-- Delete Button -->
                <button type="button" class="delete-step-btn flex-shrink-0 p-2 rounded-lg text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors">
                    <span class="material-symbols-outlined">delete</span>
                </button>
            </div>
        `;
    }

    // Add event listeners based on step type
    if (isGrouped) {
        // For grouped steps - ingredient selector syncs across group
        const ingredientSelect = card.querySelector('.ingredient-select-group');
        if (ingredientSelect) {
            ingredientSelect.addEventListener('change', (e) => {
                const ingredientId = parseInt(e.target.value) || null;
                updateIngredientGroupIngredient(step.ingredientGroupId, ingredientId);
                // Clear field error
                ingredientSelect.classList.remove('border-red-500', 'dark:border-red-500');
                const errorMsg = ingredientSelect.parentNode.querySelector('.field-error-message');
                if (errorMsg) errorMsg.remove();
            });
        }

        // Quantity input on add step
        const quantityInput = card.querySelector('.quantity-input');
        if (quantityInput) {
            quantityInput.addEventListener('change', () => updateGroupedStepFromCard(step, card));
            // Clear field error on input
            quantityInput.addEventListener('input', () => {
                quantityInput.classList.remove('border-red-500', 'dark:border-red-500');
                const errorMsg = quantityInput.parentNode.querySelector('.field-error-message');
                if (errorMsg) errorMsg.remove();
            });
        }

        // Dependencies checkboxes on add step
        const dependencyCheckboxes = card.querySelectorAll('.dependency-checkbox');
        dependencyCheckboxes.forEach(checkbox => {
            checkbox.addEventListener('change', () => updateGroupedStepDependencies(step));
        });

        // Delete button (only on first step)
        const deleteBtn = card.querySelector('.delete-step-btn');
        if (deleteBtn) {
            deleteBtn.addEventListener('click', () => {
                if (confirm(`Delete this ingredient group (3 steps)?`)) {
                    deleteStep(step.id);
                }
            });
        }
    } else {
        // Regular step event listeners
        // Action buttons - FIXED: preventDefault required to avoid blank page bug
        card.querySelectorAll('.action-btn').forEach(label => {
            label.addEventListener('click', (e) => {
                e.preventDefault();
                e.stopPropagation();

                const radio = label.querySelector('input[type="radio"]');
                if (!radio || radio.checked) return;

                // Uncheck siblings, check this one
                const groupName = radio.name;
                card.querySelectorAll(`input[name="${groupName}"]`).forEach(r => r.checked = false);
                radio.checked = true;

                // Update button styling - must also handle hover classes
                card.querySelectorAll('.action-btn').forEach(btn => {
                    btn.classList.remove('bg-primary', 'text-white');
                    btn.classList.add('bg-white', 'dark:bg-surface-dark', 'text-gray-600', 'dark:text-gray-300', 'hover:bg-gray-100', 'dark:hover:bg-surface-highlight');
                });
                label.classList.remove('bg-white', 'dark:bg-surface-dark', 'text-gray-600', 'dark:text-gray-300', 'hover:bg-gray-100', 'dark:hover:bg-surface-highlight');
                label.classList.add('bg-primary', 'text-white');

                // Show/hide parameter sections based on action
                card.querySelectorAll('[class^="param-"]').forEach(el => el.classList.add('hidden'));
                const action = radio.value;
                if (action) {
                    const cssAction = action.replace(/_/g, '-');
                    const paramDiv = card.querySelector(`.param-${cssAction}`);
                    if (paramDiv) paramDiv.classList.remove('hidden');
                    // Clear validation error
                    const actionGroup = card.querySelector('.action-group');
                    if (actionGroup) {
                        actionGroup.classList.remove('action-error');
                        const errorMsg = actionGroup.parentNode.querySelector('.action-error-message');
                        if (errorMsg) errorMsg.remove();
                    }
                }
                updateStepFromCard(step.id);
            }, true);
        });

        // Speed and direction - FIXED: preventDefault required to avoid blank page bug
        // Manually handle radio selection and styling
        card.querySelectorAll('.speed-btn, .direction-btn').forEach(label => {
            label.addEventListener('click', (e) => {
                e.preventDefault();
                e.stopPropagation();

                const radio = label.querySelector('input[type="radio"]');
                if (!radio || radio.checked) return;

                // Uncheck siblings, check this one
                const groupName = radio.name;
                card.querySelectorAll(`input[name="${groupName}"]`).forEach(r => r.checked = false);
                radio.checked = true;

                // Update button styling - must also handle hover classes
                const container = label.parentElement;
                const btnClass = label.classList.contains('speed-btn') ? '.speed-btn' : '.direction-btn';
                container.querySelectorAll(btnClass).forEach(btn => {
                    btn.classList.remove('bg-primary', 'text-white');
                    btn.classList.add('bg-white', 'dark:bg-surface-dark', 'text-gray-600', 'dark:text-gray-300', 'hover:bg-gray-100', 'dark:hover:bg-surface-highlight');
                });
                label.classList.remove('bg-white', 'dark:bg-surface-dark', 'text-gray-600', 'dark:text-gray-300', 'hover:bg-gray-100', 'dark:hover:bg-surface-highlight');
                label.classList.add('bg-primary', 'text-white');

                updateStepFromCard(step.id);
            }, true);
        });

        // Other inputs (number inputs, selects for ingredients, etc.)
        card.querySelectorAll('input:not(.action-radio):not(.speed-radio):not(.direction-radio), select, textarea').forEach(input => {
            input.addEventListener('change', () => updateStepFromCard(step.id));
            // Clear field error on input
            input.addEventListener('input', () => {
                input.classList.remove('border-red-500', 'dark:border-red-500');
                const errorMsg = input.parentNode.querySelector('.field-error-message');
                if (errorMsg) errorMsg.remove();
            });
        });

        const deleteBtn = card.querySelector('.delete-step-btn');
        if (deleteBtn) {
            deleteBtn.addEventListener('click', () => {
                if (confirm(`Delete step ${step.step_number}?`)) {
                    deleteStep(step.id);
                }
            });
        }
    }

    return card;
}

// Get ingredient name by ID
function getIngredientName(ingredientId) {
    if (!ingredientId) return null;
    const ingredients = window.existingIngredients || [];
    const ing = ingredients.find(i => i.id === parseInt(ingredientId, 10));
    return ing ? ing.name : null;
}

// Get available dependencies for any grouped step (pick, add, place)
// Excludes steps within the same ingredient group that come after this step
function getAvailableDependenciesForGroupedStep(step, currentIndex) {
    // Find the mandatory auto-dependency based on role:
    // - pick: no mandatory dependency
    // - add: pick step is mandatory
    // - place: add step is mandatory
    let mandatoryDepStepNumber = null;
    if (step.ingredientGroupRole === 'add') {
        const pickStep = steps.find(s => s.ingredientGroupId === step.ingredientGroupId && s.ingredientGroupRole === 'pick');
        mandatoryDepStepNumber = pickStep ? pickStep.step_number : null;
    } else if (step.ingredientGroupRole === 'place') {
        const addStep = steps.find(s => s.ingredientGroupId === step.ingredientGroupId && s.ingredientGroupRole === 'add');
        mandatoryDepStepNumber = addStep ? addStep.step_number : null;
    }

    // Get all steps that come before this step, excluding same-group steps
    const availableSteps = steps.filter((s, idx) => {
        // Must be before current step
        if (idx >= currentIndex) return false;
        // Exclude steps from the same ingredient group
        if (s.ingredientGroupId === step.ingredientGroupId) return false;
        return true;
    });

    if (availableSteps.length === 0) {
        return `<p class="text-xs text-gray-400 dark:text-text-secondary italic py-2">No other steps available</p>`;
    }

    // Get currently selected dependencies (excluding mandatory one)
    const selectedDeps = step.depends_on_steps.filter(num => num !== mandatoryDepStepNumber);

    return `
        <div class="dependencies-container space-y-2 max-h-32 overflow-y-auto p-3 bg-gray-50 dark:bg-surface-highlight border border-gray-300 dark:border-border-dark rounded-lg">
            ${availableSteps.reverse().slice(0, 4).map(s => `
                <label class="flex items-center gap-3 p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-border-dark cursor-pointer transition-colors group">
                    <input type="checkbox" class="dependency-checkbox w-4 h-4 rounded border-gray-300 dark:border-border-dark text-primary focus:ring-primary focus:ring-offset-0 dark:bg-surface-dark"
                           value="${s.step_number}" ${selectedDeps.includes(s.step_number) ? 'checked' : ''}>
                    <div class="flex items-center gap-2 flex-1 min-w-0">
                        <span class="flex-shrink-0 w-6 h-6 rounded-full bg-primary/10 flex items-center justify-center text-xs font-bold text-primary">${s.step_number}</span>
                        <span class="text-sm text-gray-700 dark:text-gray-300 truncate">${getActionDisplayName(s.action) || 'Untitled'}</span>
                    </div>
                </label>
            `).join('')}
        </div>
    `;
}

// Initialize sortable for drag-and-drop
function initializeSortable() {
    const container = document.getElementById('steps-container');
    if (sortableInstance) {
        sortableInstance.destroy();
    }

    sortableInstance = new Sortable(container, {
        handle: '.drag-handle',
        animation: 150,
        // Filter out system steps from being dragged
        filter: '[data-system-step="true"]',
        onMove: function (evt) {
            // Prevent dropping into first position (acquire pot must stay first)
            const relatedStepId = parseInt(evt.related.getAttribute('data-step-id'));
            const relatedStep = steps.find(s => s.id === relatedStepId);

            // Don't allow dropping before acquire pot (always step 1)
            if (relatedStep && relatedStep.action === SYSTEM_STEP_ACQUIRE) {
                return false;
            }

            return true;
        },
        onEnd: function (evt) {
            // Reorder steps array
            const movedStep = steps[evt.oldIndex];

            // Don't allow moving system steps
            if (isSystemStep(movedStep)) {
                renderSteps();
                return;
            }

            steps.splice(evt.oldIndex, 1);
            steps.splice(evt.newIndex, 0, movedStep);
            renumberSteps();

            // Ensure system steps are still in correct positions
            enforceSystemStepPositions();

            renderSteps();
            updateDAG();
        }
    });
}

// Validate dependencies to detect circular references
function validateDependencies() {
    const graph = {};
    steps.forEach(step => {
        graph[step.step_number] = step.depends_on_steps;
    });

    function hasCycle(node, visited, recStack) {
        visited[node] = true;
        recStack[node] = true;

        const dependencies = graph[node] || [];
        for (const dep of dependencies) {
            if (!visited[dep] && hasCycle(dep, visited, recStack)) {
                return true;
            } else if (recStack[dep]) {
                return true;
            }
        }

        recStack[node] = false;
        return false;
    }

    const visited = {};
    const recStack = {};
    for (const stepNum in graph) {
        if (hasCycle(parseInt(stepNum), visited, recStack)) {
            return false;
        }
    }
    return true;
}

// DAG (Directed Acyclic Graph) Visualization
// Action styling for DAG view
const DAG_COLORS = {
    'acquire_pot_from_staging': { bgClass: 'bg-gray-100 dark:bg-gray-500/20', borderClass: 'border-gray-300 dark:border-gray-500/50', textClass: 'text-gray-500 dark:text-gray-400' },
    'deliver_pot_to_serving': { bgClass: 'bg-gray-100 dark:bg-gray-500/20', borderClass: 'border-gray-300 dark:border-gray-500/50', textClass: 'text-gray-500 dark:text-gray-400' },
    'add_liquid': { bgClass: 'bg-blue-50 dark:bg-blue-500/20', borderClass: 'border-blue-200 dark:border-blue-500/50', textClass: 'text-blue-600 dark:text-blue-400' },
    'add_solid': { bgClass: 'bg-yellow-50 dark:bg-yellow-500/20', borderClass: 'border-yellow-300 dark:border-yellow-500/50', textClass: 'text-yellow-600 dark:text-yellow-400' },
    'pick_ingredient': { bgClass: 'bg-yellow-50 dark:bg-yellow-500/20', borderClass: 'border-yellow-300 dark:border-yellow-500/50', textClass: 'text-yellow-600 dark:text-yellow-400' },
    'place_ingredient': { bgClass: 'bg-yellow-50 dark:bg-yellow-500/20', borderClass: 'border-yellow-300 dark:border-yellow-500/50', textClass: 'text-yellow-600 dark:text-yellow-400' },
    'heat': { bgClass: 'bg-red-50 dark:bg-red-500/20', borderClass: 'border-red-200 dark:border-red-500/50', textClass: 'text-red-600 dark:text-red-400' },
    'agitate': { bgClass: 'bg-green-50 dark:bg-green-500/20', borderClass: 'border-green-200 dark:border-green-500/50', textClass: 'text-green-600 dark:text-green-400' },
    'open_pot_lid': { bgClass: 'bg-purple-50 dark:bg-purple-500/20', borderClass: 'border-purple-200 dark:border-purple-500/50', textClass: 'text-purple-600 dark:text-purple-400' },
    'close_pot_lid': { bgClass: 'bg-purple-50 dark:bg-purple-500/20', borderClass: 'border-purple-200 dark:border-purple-500/50', textClass: 'text-purple-600 dark:text-purple-400' }
};
const DAG_ICONS = {
    'add_liquid': 'water_drop', 'add_solid': 'add_circle', 'heat': 'local_fire_department', 'agitate': 'sync',
    'pick_ingredient': 'move_up', 'place_ingredient': 'move_down', 'open_pot_lid': 'expand_less', 'close_pot_lid': 'expand_more',
    'acquire_pot_from_staging': 'move_up', 'deliver_pot_to_serving': 'move_down'
};
const DAG_LABELS = {
    'add_liquid': 'Add Liquid', 'add_solid': 'Add Solid', 'heat': 'Heat', 'agitate': 'Agitate/Mix',
    'pick_ingredient': 'Pick', 'place_ingredient': 'Place', 'open_pot_lid': 'Open Lid', 'close_pot_lid': 'Close Lid',
    'acquire_pot_from_staging': 'Acquire Pot', 'deliver_pot_to_serving': 'Deliver Pot'
};
const DEFAULT_DAG_CONFIG = { bgClass: 'bg-gray-100 dark:bg-gray-500/20', borderClass: 'border-gray-300 dark:border-gray-500/50', textClass: 'text-gray-500 dark:text-gray-400' };

function getDAGActionConfig(action) {
    const colors = DAG_COLORS[action] || DEFAULT_DAG_CONFIG;
    return {
        icon: DAG_ICONS[action] || 'help',
        label: DAG_LABELS[action] || action || 'Untitled',
        ...colors
    };
}

// Calculate DAG levels using topological sort
function calculateDAGLevels(dagSteps) {
    const stepMap = new Map(dagSteps.map(s => [s.step_number, s]));
    const levels = new Map();

    function getLevel(stepNum) {
        if (levels.has(stepNum)) return levels.get(stepNum);

        const step = stepMap.get(stepNum);
        if (!step || !step.depends_on_steps || step.depends_on_steps.length === 0) {
            levels.set(stepNum, 0);
            return 0;
        }

        const maxDepLevel = Math.max(...step.depends_on_steps.map(d => getLevel(d)));
        const level = maxDepLevel + 1;
        levels.set(stepNum, level);
        return level;
    }

    dagSteps.forEach(s => getLevel(s.step_number));
    return levels;
}

// Group steps by level
function groupDAGByLevel(dagSteps, levels) {
    const groups = new Map();
    dagSteps.forEach(step => {
        const level = levels.get(step.step_number);
        if (!groups.has(level)) groups.set(level, []);
        groups.get(level).push(step);
    });
    return groups;
}

// Get parameter summary for DAG card display
function getDAGParamSummary(step) {
    const p = step.parameters || {};
    switch (step.action) {
        case 'add_liquid':
        case 'add_solid':
            if (p.ingredient_name) return `${p.ingredient_name} ${p.quantity || ''}${p.metric || ''}`;
            return p.quantity ? `${p.quantity}${p.metric || ''}` : '';
        case 'pick_ingredient':
        case 'place_ingredient':
            return p.ingredient_name || '';
        case 'heat':
            if (p.on_duration_sec) return `L${p.power_level || ''} ${Math.round(p.on_duration_sec / 60)}min`;
            return '';
        case 'agitate':
            if (p.duration_sec) return `${(p.speed || '').replace('_', ' ')} ${p.duration_sec}s`;
            return '';
        default:
            return '';
    }
}

// Create DAG step card HTML
function createDAGStepCard(step) {
    const config = getDAGActionConfig(step.action);
    const paramSummary = getDAGParamSummary(step);

    return `
        <div class="dag-step-card ${config.bgClass} border ${config.borderClass} rounded p-2 min-w-[160px] max-w-[200px] cursor-default transition-all hover:scale-[1.02]"
             data-dag-step="${step.step_number}" id="dag-step-${step.step_number}">
            <div class="flex items-center gap-1.5">
                <span class="flex items-center justify-center w-5 h-5 rounded-full bg-gray-200 dark:bg-surface-highlight text-[10px] font-bold text-gray-700 dark:text-white shrink-0">
                    ${step.step_number}
                </span>
                <span class="material-symbols-outlined ${config.textClass} text-base">${config.icon}</span>
                <span class="text-xs font-medium text-gray-900 dark:text-white whitespace-nowrap">${config.label}</span>
            </div>
            ${paramSummary ? `<div class="text-[11px] text-gray-600 dark:text-text-secondary mt-1 pl-6 whitespace-nowrap">${paramSummary}</div>` : ''}
        </div>
    `;
}

// Draw DAG dependency arrows using SVG
function drawDAGArrows(dagSteps) {
    const svg = document.getElementById('arrows-svg');
    const wrapper = document.getElementById('dag-wrapper');
    if (!svg || !wrapper) return;

    const wrapperRect = wrapper.getBoundingClientRect();

    // Clear existing arrows
    svg.innerHTML = '';

    // Create arrow marker
    const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
    defs.innerHTML = `
        <marker id="dag-arrowhead" markerWidth="12" markerHeight="10" refX="0" refY="5" orient="auto" markerUnits="userSpaceOnUse">
            <polygon points="0 0, 12 5, 0 10" fill="#6e56cf" opacity="0.6"/>
        </marker>
    `;
    svg.appendChild(defs);

    // Draw lines for each dependency
    dagSteps.forEach(step => {
        const deps = step.depends_on_steps || [];
        if (deps.length === 0) return;

        const targetEl = document.getElementById(`dag-step-${step.step_number}`);
        if (!targetEl) return;

        deps.forEach(depNum => {
            const sourceEl = document.getElementById(`dag-step-${depNum}`);
            if (!sourceEl) return;

            const sourceRect = sourceEl.getBoundingClientRect();
            const targetRect = targetEl.getBoundingClientRect();

            // Calculate positions relative to wrapper
            const x1 = sourceRect.left + sourceRect.width / 2 - wrapperRect.left;
            const y1 = sourceRect.bottom - wrapperRect.top;
            const x2 = targetRect.left + targetRect.width / 2 - wrapperRect.left;
            // End line 12px above the card (arrowhead is 12px long, so tip will be at card edge)
            const y2 = targetRect.top - wrapperRect.top - 12;

            // Create curved path
            const midY = (y1 + y2) / 2;
            const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
            path.setAttribute('d', `M ${x1} ${y1} C ${x1} ${midY}, ${x2} ${midY}, ${x2} ${y2}`);
            path.setAttribute('stroke', '#6e56cf');
            path.setAttribute('stroke-width', '2');
            path.setAttribute('fill', 'none');
            path.setAttribute('opacity', '0.6');
            path.setAttribute('marker-end', 'url(#dag-arrowhead)');
            svg.appendChild(path);
        });
    });
}

// Update DAG visualization
function updateDAG() {
    // Skip during form submission to avoid DOM corruption
    if (isSubmitting) return;

    const dagSection = document.getElementById('dag-section');
    const dagContainer = document.getElementById('dag-container');

    if (!dagSection || !dagContainer) return;

    if (steps.length === 0) {
        dagSection.classList.add('hidden');
        return;
    }

    dagSection.classList.remove('hidden');

    // Convert steps to DAG format
    const dagSteps = steps.map(step => ({
        step_number: step.step_number,
        action: step.action,
        parameters: step.parameters || {},
        depends_on_steps: step.depends_on_steps || []
    }));

    const levels = calculateDAGLevels(dagSteps);
    const groups = groupDAGByLevel(dagSteps, levels);
    const maxLevel = Math.max(...levels.values(), 0);

    let html = '';
    for (let level = 0; level <= maxLevel; level++) {
        const stepsAtLevel = groups.get(level) || [];
        const isLastLevel = level === maxLevel;
        html += `
            <div class="flex justify-center gap-3 ${isLastLevel ? '' : 'mb-16'}" data-dag-level="${level}">
                ${stepsAtLevel.map(s => createDAGStepCard(s)).join('')}
            </div>
        `;
    }
    dagContainer.innerHTML = html;

    // Draw arrows after DOM is updated
    requestAnimationFrame(() => drawDAGArrows(dagSteps));
}

// Redraw DAG arrows on window resize
window.addEventListener('resize', () => {
    if (document.getElementById('dag-section') && !document.getElementById('dag-section').classList.contains('hidden')) {
        const dagSteps = steps.map(step => ({
            step_number: step.step_number,
            action: step.action,
            parameters: step.parameters || {},
            depends_on_steps: step.depends_on_steps || []
        }));
        requestAnimationFrame(() => drawDAGArrows(dagSteps));
    }
});

// Use shared action configuration from recipe-actions.js
function getActionDisplayName(action) {
    if (window.RecipeActions) {
        return window.RecipeActions.getActionDisplayName(action);
    }
    return action || 'Untitled';
}

// Show validation error on an input field
function showFieldError(input, message) {
    if (!input) return;
    input.classList.add('border-red-500', 'dark:border-red-500');
    // Add error message after input's parent div
    const parent = input.parentNode;
    if (!parent.querySelector('.field-error-message')) {
        const errorMsg = document.createElement('p');
        errorMsg.className = 'field-error-message text-red-500 text-sm mt-1';
        errorMsg.textContent = message;
        parent.appendChild(errorMsg);
    }
}

// Clear all field validation errors
function clearFieldErrors() {
    document.querySelectorAll('.field-error-message').forEach(msg => msg.remove());
    document.querySelectorAll('.border-red-500').forEach(el => {
        el.classList.remove('border-red-500', 'dark:border-red-500');
    });
}

// Validate step parameters - check required fields based on action type
function validateStepParameters() {
    // Clear previous errors first
    clearFieldErrors();

    const stepCards = document.querySelectorAll('.step-card:not([data-system-step="true"])');
    const errors = [];

    for (const card of stepCards) {
        const stepId = card.getAttribute('data-step-id');
        const step = steps.find(s => s.id === parseInt(stepId));
        if (!step) continue;

        // Validate based on action type
        if (step.ingredientGroupId) {
            if (step.ingredientGroupRole === 'pick') {
                const select = card.querySelector('.ingredient-select-group');
                if (!select || !select.value) {
                    showFieldError(select, 'Please select an ingredient');
                    if (errors.length === 0) errors.push({ stepId: step.id, input: select });
                }
            } else if (step.ingredientGroupRole === 'add') {
                const qty = card.querySelector('.quantity-input');
                if (!qty || !qty.value || parseFloat(qty.value) < 0.1) {
                    showFieldError(qty, 'Please enter a quantity');
                    if (errors.length === 0) errors.push({ stepId: step.id, input: qty });
                }
            }
        } else if (step.action === 'add_liquid') {
            const select = card.querySelector('.ingredient-select-liquid');
            if (!select || !select.value) {
                showFieldError(select, 'Please select an ingredient');
                if (errors.length === 0) errors.push({ stepId: step.id, input: select });
            }
            const qty = card.querySelector('.param-add-liquid .quantity-input');
            if (!qty || !qty.value || parseFloat(qty.value) < 0.1) {
                showFieldError(qty, 'Please enter a quantity');
                if (errors.length === 0) errors.push({ stepId: step.id, input: qty });
            }
        } else if (step.action === 'heat') {
            const power = card.querySelector('.power-level-input');
            if (!power || !power.value || parseInt(power.value) < 1 || parseInt(power.value) > 10) {
                showFieldError(power, 'Please enter a power level (1-10)');
                if (errors.length === 0) errors.push({ stepId: step.id, input: power });
            }
            const duration = card.querySelector('.heat-duration-input');
            if (!duration || !duration.value || parseInt(duration.value) < 1) {
                showFieldError(duration, 'Please enter a duration');
                if (errors.length === 0) errors.push({ stepId: step.id, input: duration });
            }
        } else if (step.action === 'agitate') {
            const duration = card.querySelector('.agitate-duration-input');
            if (!duration || !duration.value || parseInt(duration.value) < 1) {
                showFieldError(duration, 'Please enter a duration');
                if (errors.length === 0) errors.push({ stepId: step.id, input: duration });
            }
        }
    }

    return errors;
}

// Flag to skip Gantt updates during form submission
let isSubmitting = false;

// Handle form submission
async function handleFormSubmit(e) {
    e.preventDefault();
    e.stopPropagation();

    // Set flag to skip Gantt updates during submission
    isSubmitting = true;

    // Update all steps from cards before submission
    for (const step of steps) {
        const card = document.querySelector(`[data-step-id="${step.id}"]`);
        if (card) {
            updateStepFromCard(step.id);
        }
    }

    // Ensure deliver step has correct dependency before saving
    updateDeliverStepDependency();

    // Validate that all non-system, non-grouped steps have an action selected
    // First, clear any previous action validation errors
    document.querySelectorAll('.action-group').forEach(group => {
        group.classList.remove('action-error');
        const existingError = group.parentNode.querySelector('.action-error-message');
        if (existingError) existingError.remove();
    });

    const stepsWithoutAction = steps.filter(step =>
        !isSystemStep(step) &&
        !step.ingredientGroupId &&
        !step.action
    );
    if (stepsWithoutAction.length > 0) {
        // Show inline error on each step missing an action
        stepsWithoutAction.forEach(step => {
            const card = document.querySelector(`[data-step-id="${step.id}"]`);
            if (card) {
                const actionGroup = card.querySelector('.action-group');
                if (actionGroup) {
                    actionGroup.classList.add('action-error');
                    // Add error message if not already present
                    if (!actionGroup.parentNode.querySelector('.action-error-message')) {
                        const errorMsg = document.createElement('p');
                        errorMsg.className = 'action-error-message text-red-500 text-sm mt-2';
                        errorMsg.textContent = 'Please select an action';
                        actionGroup.parentNode.appendChild(errorMsg);
                    }
                }
            }
        });
        // Scroll to first error
        const firstErrorCard = document.querySelector(`[data-step-id="${stepsWithoutAction[0].id}"]`);
        if (firstErrorCard) {
            firstErrorCard.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
        return false;
    }

    // Validate step parameters based on action type
    const paramErrors = validateStepParameters();
    if (paramErrors.length > 0) {
        // Scroll to first error card
        const firstErrorCard = document.querySelector(`[data-step-id="${paramErrors[0].stepId}"]`);
        if (firstErrorCard) {
            firstErrorCard.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
        // Only focus non-select inputs (Tom Select focus causes DOM corruption)
        const input = paramErrors[0].input;
        if (input && input.tagName !== 'SELECT') {
            input.focus();
        }
        return false;
    }

    // Validate dependencies
    if (!validateDependencies()) {
        alert('Circular dependency detected! Please check your step dependencies.');
        return false;
    }

    // Collect recipe basic info
    const tenantIdField = document.getElementById('tenant_id');
    const recipeData = {
        name: document.getElementById('name').value,
        estimated_prep_time_sec: parseInt(document.getElementById('estimated_prep_time_sec').value) * 60,
        estimated_cooking_time_sec: parseInt(document.getElementById('estimated_cooking_time_sec').value) * 60,
        tenant_id: tenantIdField ? tenantIdField.value : undefined
    };

    try {
        // Create or update recipe
        const recipeId = await saveRecipe(recipeData);

        // Delete all existing steps for this recipe (replace all strategy)
        await deleteAllStepsForRecipe(recipeId);

        // Save each step via API
        for (const step of steps) {
            await saveRecipeStep(recipeId, step);
        }

        // Redirect to recipes list
        window.location.href = '/recipes';
    } catch (error) {
        console.error('Error saving recipe:', error);
        document.getElementById('result').innerHTML = `
            <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4 text-red-700 dark:text-red-400">
                Error saving recipe: ${error.message}
            </div>
        `;
        // Don't redirect if there was an error
        return false;
    }

    return false;
}

async function saveRecipe(data) {
    const isEdit = document.querySelector('input[name="id"]');
    const method = isEdit ? 'PUT' : 'POST';
    const url = isEdit ? `/api/v1/recipes/${isEdit.value}` : '/api/v1/recipes';

    const response = await fetch(url, {
        method: method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });

    if (!response.ok) {
        throw new Error(`Failed to save recipe: ${response.statusText}`);
    }

    const result = await response.json();
    return result.data.id;
}

async function deleteAllStepsForRecipe(recipeId) {
    // Fetch all existing steps for this recipe
    const response = await fetch(`/api/v1/recipe-steps?recipe_id=${recipeId}`);
    if (!response.ok) {
        console.warn('Failed to fetch existing steps for deletion:', response.status);
        return;
    }

    const existingSteps = await response.json();
    if (!existingSteps || !existingSteps.data) {
        return;
    }

    // Delete each step individually
    for (const step of existingSteps.data) {
        try {
            const deleteResponse = await fetch(`/api/v1/recipe-steps/${step.id}`, {
                method: 'DELETE'
            });
            if (!deleteResponse.ok) {
                console.warn(`Failed to delete step ${step.id}:`, deleteResponse.status);
            }
        } catch (error) {
            console.warn(`Error deleting step ${step.id}:`, error);
        }
    }
}

async function saveRecipeStep(recipeId, step) {
    const stepData = {
        recipe_id: recipeId,
        step_number: step.step_number,
        action: step.action,
        parameters: JSON.stringify(step.parameters),
        depends_on_steps: JSON.stringify(step.depends_on_steps)
    };

    const response = await fetch('/api/v1/recipe-steps', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(stepData)
    });

    if (!response.ok) {
        const errorText = await response.text();
        console.error(`Failed to save step ${step.step_number}:`, response.status, errorText);
        throw new Error(`Failed to save step ${step.step_number}: ${response.statusText}`);
    }

    return await response.json();
}
